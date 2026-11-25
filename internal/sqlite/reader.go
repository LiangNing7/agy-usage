// Package sqlite implements a zero-CGO, zero-dependency, read-only SQLite3
// table B-tree scanner supporting overflow pages and WAL (Write-Ahead Log)
// committed frame overlays, strictly adhering to https://www.sqlite.org/fileformat2.html.
package sqlite

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"unicode/utf16"
)

var sqliteMagic = []byte("SQLite format 3\x00")

// Value represents a decoded SQLite cell column value (nil, int64, float64, string, or []byte).
type Value any

// Row maps lowercase column names to their decoded values.
type Row map[string]Value

func (r Row) String(col string) string {
	v, ok := r[strings.ToLower(col)]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

func (r Row) Int64(col string) int64 {
	v, ok := r[strings.ToLower(col)]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	default:
		return 0
	}
}

type dbReader struct {
	file         *os.File
	fileSize     int64
	pageSize     int
	usableSize   int
	textEncoding uint32 // 1=UTF8, 2=UTF16LE, 3=UTF16BE
	walPages     map[uint32][]byte
	walDBSize    uint32 // non-zero if WAL committed a specific page count
}

// ReadTable opens dbPath read-only (including any committed frames in dbPath+"-wal")
// and returns all rows from tableName.
func ReadTable(dbPath, tableName string) ([]Row, error) {
	f, err := os.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() < 100 {
		return nil, errors.New("sqlite: file smaller than 100-byte header")
	}

	r := &dbReader{
		file:     f,
		fileSize: st.Size(),
		walPages: make(map[uint32][]byte),
	}

	// Read initial 100-byte header from main DB file to bootstrap page size,
	// then reload WAL (which might even override page 1!).
	hdr := make([]byte, 100)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return nil, err
	}
	if err := r.parseHeader(hdr); err != nil {
		return nil, err
	}

	// Load committed WAL frames if <dbPath>-wal exists.
	r.loadWAL(dbPath + "-wal")
	if p1, ok := r.walPages[1]; ok && len(p1) >= 100 {
		if err := r.parseHeader(p1[:100]); err != nil {
			return nil, err
		}
	}

	// Scan sqlite_schema (rooted at page 1) to locate tableName.
	var rootPage uint32
	var createSQL string
	found := false
	err = r.scanTableBTree(1, make(map[uint32]bool), func(_ int64, vals []Value) error {
		if len(vals) < 5 {
			return nil
		}
		typ, _ := asString(vals[0])
		name, _ := asString(vals[1])
		if strings.EqualFold(typ, "table") && strings.EqualFold(name, tableName) {
			if rp, ok := vals[3].(int64); ok && rp > 0 {
				rootPage = uint32(rp)
				createSQL, _ = asString(vals[4])
				found = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found || rootPage == 0 {
		return nil, fmt.Errorf("sqlite: table %q not found", tableName)
	}

	cols, intPKIdx := parseTableColumns(createSQL)
	if len(cols) == 0 {
		return nil, fmt.Errorf("sqlite: could not parse columns for %q", tableName)
	}

	var rows []Row
	err = r.scanTableBTree(rootPage, make(map[uint32]bool), func(rowID int64, vals []Value) error {
		row := make(Row, len(cols))
		for i, col := range cols {
			if i == intPKIdx && (i >= len(vals) || vals[i] == nil) {
				row[col] = rowID
				continue
			}
			if i < len(vals) {
				row[col] = vals[i]
			} else {
				row[col] = nil
			}
		}
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *dbReader) parseHeader(hdr []byte) error {
	if !bytes.Equal(hdr[:16], sqliteMagic) {
		return errors.New("sqlite: invalid file header magic")
	}
	ps := int(binary.BigEndian.Uint16(hdr[16:18]))
	if ps == 1 {
		ps = 65536
	}
	if ps < 512 || (ps&(ps-1)) != 0 {
		return fmt.Errorf("sqlite: invalid page size %d", ps)
	}
	reserved := int(hdr[20])
	usable := ps - reserved
	if usable < 480 {
		return fmt.Errorf("sqlite: invalid usable page size %d", usable)
	}
	enc := binary.BigEndian.Uint32(hdr[56:60])
	if enc == 0 {
		enc = 1
	}
	r.pageSize = ps
	r.usableSize = usable
	r.textEncoding = enc
	return nil
}

func (r *dbReader) readPage(pageNo uint32) ([]byte, error) {
	if pageNo == 0 {
		return nil, errors.New("sqlite: invalid page number 0")
	}
	if r.walDBSize > 0 && pageNo > r.walDBSize {
		return nil, fmt.Errorf("sqlite: page %d beyond WAL db size %d", pageNo, r.walDBSize)
	}
	if data, ok := r.walPages[pageNo]; ok {
		return data, nil
	}
	off := int64(pageNo-1) * int64(r.pageSize)
	if off+int64(r.pageSize) > r.fileSize {
		return nil, fmt.Errorf("sqlite: page %d out of bounds", pageNo)
	}
	buf := make([]byte, r.pageSize)
	if _, err := r.file.ReadAt(buf, off); err != nil {
		return nil, err
	}
	return buf, nil
}

// loadWAL overlays committed frames from a SQLite3 WAL file if present and valid.
func (r *dbReader) loadWAL(walPath string) {
	wf, err := os.Open(walPath)
	if err != nil {
		return
	}
	defer wf.Close()

	st, err := wf.Stat()
	if err != nil || st.Size() <= 32 {
		return
	}

	var walHdr [32]byte
	if _, err := io.ReadFull(wf, walHdr[:]); err != nil {
		return
	}
	magic := binary.BigEndian.Uint32(walHdr[0:4])
	var bigEndCksum bool
	switch magic {
	case 0x377f0682:
		bigEndCksum = false
	case 0x377f0683:
		bigEndCksum = true
	default:
		return
	}

	walPS := int(binary.BigEndian.Uint32(walHdr[8:12]))
	if walPS == 1 {
		walPS = 65536
	}
	if walPS != r.pageSize {
		return
	}

	s0, s1 := walChecksum(walHdr[0:24], 0, 0, bigEndCksum)
	if s0 != binary.BigEndian.Uint32(walHdr[24:28]) || s1 != binary.BigEndian.Uint32(walHdr[28:32]) {
		return
	}
	salt1 := binary.BigEndian.Uint32(walHdr[16:20])
	salt2 := binary.BigEndian.Uint32(walHdr[20:24])

	pending := make(map[uint32][]byte)
	var frameHdr [24]byte
	frameBuf := make([]byte, walPS)

	for {
		if _, err := io.ReadFull(wf, frameHdr[:]); err != nil {
			break
		}
		if _, err := io.ReadFull(wf, frameBuf); err != nil {
			break
		}
		if binary.BigEndian.Uint32(frameHdr[8:12]) != salt1 || binary.BigEndian.Uint32(frameHdr[12:16]) != salt2 {
			break
		}
		s0, s1 = walChecksum(frameHdr[0:8], s0, s1, bigEndCksum)
		s0, s1 = walChecksum(frameBuf, s0, s1, bigEndCksum)
		if s0 != binary.BigEndian.Uint32(frameHdr[16:20]) || s1 != binary.BigEndian.Uint32(frameHdr[20:24]) {
			break
		}

		pgNo := binary.BigEndian.Uint32(frameHdr[0:4])
		commitSize := binary.BigEndian.Uint32(frameHdr[4:8])
		if pgNo > 0 {
			cp := make([]byte, walPS)
			copy(cp, frameBuf)
			pending[pgNo] = cp
		}
		if commitSize > 0 {
			for k, v := range pending {
				if k <= commitSize {
					r.walPages[k] = v
				}
			}
			for k := range r.walPages {
				if k > commitSize {
					delete(r.walPages, k)
				}
			}
			r.walDBSize = commitSize
			pending = make(map[uint32][]byte)
		}
	}
}

func walChecksum(data []byte, s0, s1 uint32, bigEnd bool) (uint32, uint32) {
	for i := 0; i+8 <= len(data); i += 8 {
		var w0, w1 uint32
		if bigEnd {
			w0 = binary.BigEndian.Uint32(data[i : i+4])
			w1 = binary.BigEndian.Uint32(data[i+4 : i+8])
		} else {
			w0 = binary.LittleEndian.Uint32(data[i : i+4])
			w1 = binary.LittleEndian.Uint32(data[i+4 : i+8])
		}
		s0 += w0 + s1
		s1 += w1 + s0
	}
	return s0, s1
}

func (r *dbReader) scanTableBTree(pageNo uint32, visited map[uint32]bool, onRow func(rowID int64, vals []Value) error) error {
	if visited[pageNo] {
		return nil
	}
	visited[pageNo] = true

	page, err := r.readPage(pageNo)
	if err != nil {
		return err
	}
	hdrOff := 0
	if pageNo == 1 {
		hdrOff = 100
	}
	if hdrOff+8 > len(page) {
		return errors.New("sqlite: truncated b-tree page header")
	}

	pageType := page[hdrOff]
	cellCount := int(binary.BigEndian.Uint16(page[hdrOff+3 : hdrOff+5]))

	switch pageType {
	case 0x05: // Interior Table B-Tree
		if hdrOff+12+cellCount*2 > r.usableSize {
			return errors.New("sqlite: interior page cell pointers out of bounds")
		}
		rightMost := binary.BigEndian.Uint32(page[hdrOff+8 : hdrOff+12])
		ptrBase := hdrOff + 12
		for i := 0; i < cellCount; i++ {
			cellOff := int(binary.BigEndian.Uint16(page[ptrBase+i*2 : ptrBase+i*2+2]))
			if cellOff+4 > r.usableSize {
				return errors.New("sqlite: interior cell offset out of bounds")
			}
			leftChild := binary.BigEndian.Uint32(page[cellOff : cellOff+4])
			if err := r.scanTableBTree(leftChild, visited, onRow); err != nil {
				return err
			}
		}
		return r.scanTableBTree(rightMost, visited, onRow)

	case 0x0d: // Leaf Table B-Tree
		if hdrOff+8+cellCount*2 > r.usableSize {
			return errors.New("sqlite: leaf page cell pointers out of bounds")
		}
		ptrBase := hdrOff + 8
		for i := 0; i < cellCount; i++ {
			cellOff := int(binary.BigEndian.Uint16(page[ptrBase+i*2 : ptrBase+i*2+2]))
			if cellOff >= r.usableSize {
				return errors.New("sqlite: leaf cell offset out of bounds")
			}
			payloadSize, n1 := readVarint(page[cellOff:r.usableSize])
			if n1 == 0 {
				return errors.New("sqlite: invalid payload size varint")
			}
			rowIDU, n2 := readVarint(page[cellOff+n1 : r.usableSize])
			if n2 == 0 {
				return errors.New("sqlite: invalid rowid varint")
			}
			payloadStart := cellOff + n1 + n2
			payload, err := r.readCellPayload(page, payloadStart, int(payloadSize))
			if err != nil {
				return err
			}
			vals, err := r.decodeRecord(payload)
			if err != nil {
				return err
			}
			if err := onRow(int64(rowIDU), vals); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("sqlite: unexpected b-tree page type 0x%02x on page %d", pageType, pageNo)
	}
}

func (r *dbReader) readCellPayload(page []byte, start int, totalSize int) ([]byte, error) {
	if totalSize == 0 {
		return nil, nil
	}
	if totalSize < 0 {
		return nil, errors.New("sqlite: negative payload size")
	}
	u := r.usableSize
	x := u - 35
	if totalSize <= x {
		if start+totalSize > u {
			return nil, errors.New("sqlite: inline cell payload exceeds usable page")
		}
		out := make([]byte, totalSize)
		copy(out, page[start:start+totalSize])
		return out, nil
	}

	m := ((u - 12) * 32 / 255) - 23
	k := m + ((totalSize - m) % (u - 4))
	localSize := k
	if k > x {
		localSize = m
	}
	if start+localSize+4 > u {
		return nil, errors.New("sqlite: overflow pointer exceeds usable page")
	}

	out := make([]byte, totalSize)
	copy(out, page[start:start+localSize])
	copied := localSize
	ovfPage := binary.BigEndian.Uint32(page[start+localSize : start+localSize+4])

	seenOvf := make(map[uint32]bool)
	for copied < totalSize && ovfPage != 0 {
		if seenOvf[ovfPage] {
			return nil, errors.New("sqlite: cycle detected in overflow chain")
		}
		seenOvf[ovfPage] = true
		op, err := r.readPage(ovfPage)
		if err != nil {
			return nil, err
		}
		nextOvf := binary.BigEndian.Uint32(op[0:4])
		chunk := u - 4
		if rem := totalSize - copied; rem < chunk {
			chunk = rem
		}
		copy(out[copied:copied+chunk], op[4:4+chunk])
		copied += chunk
		ovfPage = nextOvf
	}
	if copied < totalSize {
		return nil, errors.New("sqlite: truncated overflow chain")
	}
	return out, nil
}

func (r *dbReader) decodeRecord(payload []byte) ([]Value, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	hdrSizeU, pos := readVarint(payload)
	if pos == 0 || int(hdrSizeU) > len(payload) {
		return nil, errors.New("sqlite: invalid record header size")
	}
	hdrEnd := int(hdrSizeU)
	var serialTypes []uint64
	for pos < hdrEnd {
		st, n := readVarint(payload[pos:hdrEnd])
		if n == 0 {
			return nil, errors.New("sqlite: invalid serial type varint")
		}
		serialTypes = append(serialTypes, st)
		pos += n
	}

	bodyPos := hdrEnd
	vals := make([]Value, len(serialTypes))
	for i, st := range serialTypes {
		switch st {
		case 0:
			vals[i] = nil
		case 1:
			if bodyPos+1 > len(payload) {
				return nil, errors.New("sqlite: truncated int8")
			}
			vals[i] = int64(int8(payload[bodyPos]))
			bodyPos += 1
		case 2:
			if bodyPos+2 > len(payload) {
				return nil, errors.New("sqlite: truncated int16")
			}
			vals[i] = int64(int16(binary.BigEndian.Uint16(payload[bodyPos : bodyPos+2])))
			bodyPos += 2
		case 3:
			if bodyPos+3 > len(payload) {
				return nil, errors.New("sqlite: truncated int24")
			}
			v := uint32(payload[bodyPos])<<24 | uint32(payload[bodyPos+1])<<16 | uint32(payload[bodyPos+2])<<8
			vals[i] = int64(int32(v) >> 8)
			bodyPos += 3
		case 4:
			if bodyPos+4 > len(payload) {
				return nil, errors.New("sqlite: truncated int32")
			}
			vals[i] = int64(int32(binary.BigEndian.Uint32(payload[bodyPos : bodyPos+4])))
			bodyPos += 4
		case 5:
			if bodyPos+6 > len(payload) {
				return nil, errors.New("sqlite: truncated int48")
			}
			b := payload[bodyPos : bodyPos+6]
			v := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 |
				uint64(b[3])<<32 | uint64(b[4])<<24 | uint64(b[5])<<16
			vals[i] = int64(v) >> 16
			bodyPos += 6
		case 6:
			if bodyPos+8 > len(payload) {
				return nil, errors.New("sqlite: truncated int64")
			}
			vals[i] = int64(binary.BigEndian.Uint64(payload[bodyPos : bodyPos+8]))
			bodyPos += 8
		case 7:
			if bodyPos+8 > len(payload) {
				return nil, errors.New("sqlite: truncated float64")
			}
			vals[i] = math.Float64frombits(binary.BigEndian.Uint64(payload[bodyPos : bodyPos+8]))
			bodyPos += 8
		case 8:
			vals[i] = int64(0)
		case 9:
			vals[i] = int64(1)
		case 10, 11:
			return nil, errors.New("sqlite: unsupported internal serial type")
		default:
			if st%2 == 0 {
				blen := int((st - 12) / 2)
				if bodyPos+blen > len(payload) {
					return nil, errors.New("sqlite: truncated blob")
				}
				cp := make([]byte, blen)
				copy(cp, payload[bodyPos:bodyPos+blen])
				vals[i] = cp
				bodyPos += blen
			} else {
				slen := int((st - 13) / 2)
				if bodyPos+slen > len(payload) {
					return nil, errors.New("sqlite: truncated text")
				}
				vals[i] = r.decodeText(payload[bodyPos : bodyPos+slen])
				bodyPos += slen
			}
		}
	}
	return vals, nil
}

func (r *dbReader) decodeText(b []byte) string {
	switch r.textEncoding {
	case 2: // UTF-16LE
		u16 := make([]uint16, len(b)/2)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
		}
		return string(utf16.Decode(u16))
	case 3: // UTF-16BE
		u16 := make([]uint16, len(b)/2)
		for i := range u16 {
			u16[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
		}
		return string(utf16.Decode(u16))
	default:
		return string(b)
	}
}

func readVarint(buf []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(buf) && i < 9; i++ {
		b := buf[i]
		if i == 8 {
			v = (v << 8) | uint64(b)
			return v, 9
		}
		v = (v << 7) | uint64(b&0x7f)
		if b&0x80 == 0 {
			return v, i + 1
		}
	}
	return 0, 0
}

func asString(v Value) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case []byte:
		return string(x), true
	default:
		return "", false
	}
}

// parseTableColumns extracts lowercase column names and detects INTEGER PRIMARY KEY alias index.
func parseTableColumns(createSQL string) ([]string, int) {
	openIdx := strings.IndexByte(createSQL, '(')
	if openIdx < 0 {
		return nil, -1
	}
	upperTail := strings.ToUpper(createSQL[openIdx:])
	withoutRowID := strings.Contains(upperTail, "WITHOUT ROWID")

	// Find matching closing paren for column list.
	depth := 0
	inQuote := byte(0)
	closeIdx := -1
	for i := openIdx; i < len(createSQL); i++ {
		ch := createSQL[i]
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			inQuote = ch
		case '[':
			inQuote = ']'
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx <= openIdx {
		return nil, -1
	}

	body := createSQL[openIdx+1 : closeIdx]
	parts := splitTopLevelCommas(body)

	var cols []string
	intPKIdx := -1
	for _, rawPart := range parts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}
		name, rest := scanSQLIdentifier(part)
		if name == "" {
			continue
		}
		upName := strings.ToUpper(name)
		if (upName == "PRIMARY" || upName == "UNIQUE" || upName == "CHECK" ||
			upName == "FOREIGN" || upName == "CONSTRAINT") && !isQuotedIdent(part) {
			continue
		}
		colIdx := len(cols)
		cols = append(cols, strings.ToLower(name))
		upDef := strings.ToUpper(strings.Join(strings.Fields(rest), " "))
		if !withoutRowID && strings.HasPrefix(upDef, "INTEGER ") && strings.Contains(upDef, "PRIMARY KEY") && !strings.Contains(upDef, "DESC") {
			intPKIdx = colIdx
		}
	}
	return cols, intPKIdx
}

func isQuotedIdent(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case '`', '"', '[', '\'':
		return true
	default:
		return false
	}
}

func scanSQLIdentifier(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	q := s[0]
	if q == '`' || q == '"' || q == '\'' || q == '[' {
		endCh := q
		if q == '[' {
			endCh = ']'
		}
		for i := 1; i < len(s); i++ {
			if s[i] == endCh {
				return s[1:i], s[i+1:]
			}
		}
		return "", ""
	}
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' && s[i] != '(' {
		i++
	}
	return s[:i], s[i:]
}

func splitTopLevelCommas(s string) []string {
	var out []string
	depth := 0
	inQuote := byte(0)
	start := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			inQuote = ch
		case '[':
			inQuote = ']'
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}
