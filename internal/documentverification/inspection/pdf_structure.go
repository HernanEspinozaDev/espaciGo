package inspection

import (
	"bytes"
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const (
	pdfParseTimeout                = 5 * time.Second
	pdfCommandTimeout              = 5 * time.Second
	maxParserOutputBytes           = 1 << 20
	maxPDFDecodedStream            = int64(128 << 20)
	maxPDFPages                    = 20
	maxPDFObjects                  = 100_000
	maxPDFObjectStreams            = 10_000
	maxPDFObjectProlog             = int64(1 << 20)
	maxPDFRecursion                = 64
	maxPDFStructureNodes           = 500_000
	maxPDFDecodedTotal             = int64(256 << 20)
	pdfDecodeMaterializationFactor = int64(4)
)

var (
	errPDFActiveContent  = errors.New("active PDF content rejected")
	errParserOutputLimit = errors.New("PDF parser output limit exceeded")
)

type boundedOutput struct{ buffer bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > maxParserOutputBytes-b.buffer.Len() {
		return 0, errParserOutputLimit
	}
	return b.buffer.Write(p)
}

func (b *boundedOutput) String() string { return b.buffer.String() }

var forbiddenPDFKeys = [...]string{
	"OpenAction", "AA", "JavaScript", "JS", "AcroForm", "XFA", "NeedAppearances",
	"EmbeddedFiles", "EmbeddedFile", "Filespec", "FileSpec", "EF", "AF", "FT",
	"RichMedia", "3D", "Sound", "Movie", "Rendition",
}

var forbiddenPDFActions = map[string]struct{}{
	"GoTo": {}, "GoToR": {}, "GoToE": {}, "Launch": {}, "Thread": {}, "URI": {},
	"Sound": {}, "Movie": {}, "Hide": {}, "Named": {}, "SubmitForm": {}, "ResetForm": {},
	"ImportData": {}, "JavaScript": {}, "SetOCGState": {}, "Rendition": {}, "Trans": {},
	"GoTo3DView": {},
}

func validatePDFStructure(data []byte, maxPixels int64, expectedPages int) error {
	return validatePDFStructureWithBudget(data, maxPixels, expectedPages, maxPDFDecodedTotal)
}

func validatePDFStructureWithBudget(data []byte, maxPixels int64, expectedPages int, decodedBudget int64) error {
	if len(data) == 0 || len(data) > maxBytes || maxPixels <= 0 || expectedPages < 1 || decodedBudget <= 0 {
		return ErrRejected
	}
	if expectedPages > maxPDFPages {
		return ErrRejected
	}
	parserDecodeLimit, scannerDecodeLimit, scannerBudget, err := pdfDecodeLimits(data, decodedBudget, expectedPages)
	if err != nil {
		return ErrRejected
	}

	conf := model.NewStatelessConfiguration()
	conf.CheckFileNameExt = false
	conf.Reader15 = true
	conf.ValidationMode = model.ValidationStrict
	conf.ValidateLinks = false
	conf.Offline = true
	conf.Optimize = false
	conf.OptimizeBeforeWriting = false
	conf.Limits.MaxInputBytes = maxBytes
	conf.Limits.MaxObjectBytes = maxBytes
	conf.Limits.MaxStreamBytes = maxBytes
	conf.Limits.MaxDecodeBytes = parserDecodeLimit
	conf.Limits.MaxImagePixels = maxPixels
	if maxPixels > math.MaxInt64/4 {
		conf.Limits.MaxImageBytes = math.MaxInt64
	} else {
		conf.Limits.MaxImageBytes = maxPixels * 4
	}
	conf.Limits.MaxObjectCount = maxPDFObjects
	conf.Limits.MaxObjectStreamCount = maxPDFObjectStreams
	conf.Limits.MaxObjectStreamFirst = maxPDFObjectProlog
	if conf.Limits.MaxObjectStreamFirst > parserDecodeLimit {
		conf.Limits.MaxObjectStreamFirst = parserDecodeLimit
	}
	conf.Limits.MaxXRefEntries = maxPDFObjects
	conf.Limits.MaxRecursionDepth = maxPDFRecursion
	conf.Cmd = model.VALIDATE

	ctx, cancel := context.WithTimeout(context.Background(), pdfParseTimeout)
	defer cancel()
	parserBudget := decodedBudget - scannerBudget
	parsed, _, err := readAndValidatePDFWithinBudget(ctx, data, conf, parserDecodeLimit, parserBudget)
	if err != nil || parsed == nil || parsed.XRefTable == nil || parsed.Encrypt != nil || parsed.PageCount != expectedPages {
		return ErrRejected
	}

	scanner := pdfStructureScanner{
		ctx:                  ctx,
		xref:                 parsed.XRefTable,
		visited:              make(map[pdfObjectRef]struct{}, len(parsed.Table)),
		decodedObjectStreams: make(map[int]struct{}),
		decodedByteLimit:     scannerBudget,
		decoderStageLimit:    scannerDecodeLimit,
	}
	for objectNumber, entry := range parsed.Table {
		if entry == nil || entry.Free {
			continue
		}
		generation := 0
		if entry.Generation != nil {
			generation = *entry.Generation
		}
		ref := types.IndirectRef{
			ObjectNumber:     types.Integer(objectNumber),
			GenerationNumber: types.Integer(generation),
		}
		if err := scanner.walk(ref, 0); err != nil {
			return err
		}
	}
	return nil
}

type pdfParserBudgetUsage struct {
	readReserve     int64
	predecodedBytes int64
}

func (u pdfParserBudgetUsage) total() int64 {
	total, ok := reserveDecodedPDFBytes(u.readReserve, u.predecodedBytes, math.MaxInt64)
	if !ok {
		return math.MaxInt64
	}
	return total
}

func readAndValidatePDFWithinBudget(ctx context.Context, data []byte, conf *model.Configuration, parserLimit, parserBudget int64) (parsed *model.Context, usage pdfParserBudgetUsage, err error) {
	defer func() {
		if recover() != nil {
			parsed = nil
			err = ErrRejected
		}
	}()
	if ctx == nil || ctx.Err() != nil || len(data) == 0 || conf == nil || parserLimit <= 0 || parserBudget <= 0 {
		return nil, usage, ErrRejected
	}

	usage.readReserve, err = pdfParserReadReserve(data, parserLimit, parserBudget)
	if err != nil || usage.readReserve > parserBudget {
		return nil, usage, ErrRejected
	}
	parsed, err = api.ReadContext(ctx, bytes.NewReader(data), conf)
	if err != nil || parsed == nil || parsed.XRefTable == nil || parsed.Encrypt != nil {
		return nil, usage, ErrRejected
	}

	remaining := parserBudget - usage.readReserve
	usage.predecodedBytes, err = predecodePDFStreams(ctx, parsed, parserLimit, &remaining)
	if err != nil || usage.total() > parserBudget {
		return nil, usage, ErrRejected
	}
	if err := api.ValidateContext(ctx, parsed); err != nil {
		return nil, usage, ErrRejected
	}
	return parsed, usage, nil
}

// pdfParserReadReserve covers pdfcpu's pre-validation xref/object-stream reads,
// including object-stream prologs and one full decode of each object stream.
// Normal streams are then decoded once under the remaining aggregate budget
// and cached in the xref table before ValidateContext can revisit them.
func pdfParserReadReserve(data []byte, parserLimit, parserBudget int64) (int64, error) {
	if parserLimit <= 0 || parserBudget < 0 {
		return 0, ErrRejected
	}
	filterStages, err := countPDFDecoderStages(data)
	if err != nil {
		return 0, err
	}
	filterPasses, ok := pdfDecodedByteProduct(filterStages, 2)
	if !ok {
		return 0, ErrRejected
	}
	passCount, ok := reserveDecodedPDFBytes(filterPasses, countPDFStreamMarkers(data), math.MaxInt64)
	if !ok {
		return 0, ErrRejected
	}
	reserve, ok := pdfDecodedByteProduct(passCount, parserLimit)
	if !ok || reserve > parserBudget {
		return 0, ErrRejected
	}
	return reserve, nil
}

func pdfDecodedByteProduct(count, bytesPerItem int64) (int64, bool) {
	if count < 0 || bytesPerItem < 0 || count > 0 && bytesPerItem > math.MaxInt64/count {
		return 0, false
	}
	return count * bytesPerItem, true
}

func predecodePDFStreams(ctx context.Context, parsed *model.Context, parserLimit int64, remaining *int64) (int64, error) {
	if ctx == nil || parsed == nil || parsed.XRefTable == nil || parserLimit <= 0 || remaining == nil || *remaining < 0 {
		return 0, ErrRejected
	}
	objectNumbers := make([]int, 0, len(parsed.Table))
	for objectNumber := range parsed.Table {
		objectNumbers = append(objectNumbers, objectNumber)
	}
	sort.Ints(objectNumbers)

	var used int64
	for _, objectNumber := range objectNumbers {
		if ctx.Err() != nil {
			return 0, ErrRejected
		}
		entry := parsed.Table[objectNumber]
		if entry == nil || entry.Free {
			continue
		}
		decodeAndStore := func(stream *types.StreamDict) error {
			decoded, err := predecodePDFStreamStages(stream, parserLimit, remaining)
			if err != nil {
				return err
			}
			var ok bool
			used, ok = reserveDecodedPDFBytes(used, decoded, math.MaxInt64)
			if !ok {
				return ErrRejected
			}
			return nil
		}
		switch stream := entry.Object.(type) {
		case types.StreamDict:
			if err := decodeAndStore(&stream); err != nil {
				return 0, ErrRejected
			}
			entry.Object = stream
		case *types.StreamDict:
			if stream == nil {
				return 0, ErrRejected
			}
			copy := *stream
			if err := decodeAndStore(&copy); err != nil {
				return 0, ErrRejected
			}
			entry.Object = copy
		case types.XRefStreamDict:
			if err := decodeAndStore(&stream.StreamDict); err != nil {
				return 0, ErrRejected
			}
			entry.Object = stream
		case *types.XRefStreamDict:
			if stream == nil {
				return 0, ErrRejected
			}
			if err := decodeAndStore(&stream.StreamDict); err != nil {
				return 0, ErrRejected
			}
		case types.ObjectStreamDict, *types.ObjectStreamDict:
			// pdfcpu's lazy objects share one ObjectStreamDict pointer; its full
			// decode is covered by the pre-validation reserve above.
		}
	}
	return used, nil
}

func predecodePDFStreamStages(stream *types.StreamDict, parserLimit int64, remaining *int64) (int64, error) {
	if stream == nil || parserLimit <= 0 || remaining == nil || *remaining < 0 {
		return 0, ErrRejected
	}
	if stream.Content != nil {
		return 0, nil
	}
	filters := stream.FilterPipeline
	if len(filters) > 0 && filters[0].Name == "Crypt" {
		filters = filters[1:]
	}
	if len(filters) == 0 || len(filters) == 1 &&
		((filters[0].Name == filter.DCT && stream.CSComponents != 4) || filters[0].Name == filter.JPX || filters[0].Name == filter.JBIG2) {
		stream.Content = stream.Raw
		if stream.Content == nil {
			stream.Content = []byte{}
		}
		return 0, nil
	}

	current := stream.Raw
	var used int64
	for index, pdfFilter := range filters {
		if pdfFilter.Name == filter.JPX || pdfFilter.Name == filter.JBIG2 {
			if index != len(filters)-1 {
				return used, ErrRejected
			}
			continue
		}
		if *remaining <= 0 {
			return used, ErrRejected
		}
		stageLimit := parserLimit
		if *remaining < stageLimit {
			stageLimit = *remaining
		}
		stage := *stream
		stage.Raw = current
		stage.Content = nil
		stage.FilterPipeline = []types.PDFFilter{pdfFilter}
		if err := stage.DecodeWithLimit(stageLimit); err != nil {
			return used, ErrRejected
		}
		decodedBytes := int64(len(stage.Content))
		if decodedBytes > stageLimit || decodedBytes > *remaining {
			return used, ErrRejected
		}
		var ok bool
		used, ok = reserveDecodedPDFBytes(used, decodedBytes, math.MaxInt64)
		if !ok {
			return used, ErrRejected
		}
		*remaining -= decodedBytes
		current = stage.Content
	}
	if current == nil {
		current = []byte{}
	}
	stream.Content = current
	return used, nil
}

type pdfObjectRef struct {
	objectNumber     int64
	generationNumber int64
}

type decodedPDFObjectStream struct {
	objectNumbers []int
	objects       []types.Object
}

type pdfStructureScanner struct {
	ctx                  context.Context
	xref                 *model.XRefTable
	visited              map[pdfObjectRef]struct{}
	decodedObjectStreams map[int]struct{}
	objectStreams        map[int]decodedPDFObjectStream
	decodedBytes         int64
	decodedByteLimit     int64
	decoderStageLimit    int64
	walkNodes            int
}

// pdfDecodeLimits partitions one document-wide byte budget between pdfcpu's
// parse/validation work and the later structural object-stream walk. The raw
// token count is deliberately an upper bound: filter names inside strings or
// stream payloads only make the per-stage allowance more conservative.
func pdfDecodeLimits(data []byte, total int64, expectedPages int) (parserLimit, scannerStageLimit, scannerBudget int64, err error) {
	if total < 2 || expectedPages < 1 || expectedPages > maxPDFPages {
		return 0, 0, 0, ErrRejected
	}
	filterStages, err := countPDFDecoderStages(data)
	if err != nil {
		return 0, 0, 0, err
	}
	streamMarkers := countPDFStreamMarkers(data)
	operations := filterStages + streamMarkers
	parserBudget := total - total/4
	scannerBudget = total - parserBudget
	parserDenominator := operations * pdfDecodeMaterializationFactor
	parserDenominator += int64(expectedPages) * 2
	if parserDenominator == 0 {
		parserDenominator = 1
	}
	if operations == 0 {
		parserLimit = parserBudget / int64(expectedPages*2)
		scannerStageLimit = scannerBudget
	} else {
		parserLimit = parserBudget / parserDenominator
		scannerStageLimit = scannerBudget / (operations * pdfDecodeMaterializationFactor)
	}
	if parserLimit > maxPDFDecodedStream {
		parserLimit = maxPDFDecodedStream
	}
	if scannerStageLimit > maxPDFDecodedStream {
		scannerStageLimit = maxPDFDecodedStream
	}
	if parserLimit <= 0 || scannerStageLimit <= 0 {
		return 0, 0, 0, ErrRejected
	}
	return parserLimit, scannerStageLimit, scannerBudget, nil
}

func countPDFDecoderStages(data []byte) (int64, error) {
	var count int64
	for i := 0; i < len(data); {
		if data[i] != '/' {
			i++
			continue
		}
		name, end := readPDFName(data, i)
		if name == "Filter" && hasIndirectPDFFilter(data, end) {
			return 0, ErrRejected
		}
		if isPDFDecoderName(name) {
			count++
		}
		if end <= i {
			i++
		} else {
			i = end
		}
	}
	return count, nil
}

func readPDFName(data []byte, slash int) (string, int) {
	var name strings.Builder
	for i := slash + 1; i < len(data); i++ {
		if isPDFDelimiter(data[i]) {
			return name.String(), i
		}
		if data[i] == '#' && i+2 < len(data) {
			if decoded, ok := decodePDFNameEscape(data[i+1], data[i+2]); ok {
				name.WriteByte(decoded)
				i += 2
				continue
			}
		}
		name.WriteByte(data[i])
	}
	return name.String(), len(data)
}

func decodePDFNameEscape(high, low byte) (byte, bool) {
	value := func(b byte) (byte, bool) {
		switch {
		case b >= '0' && b <= '9':
			return b - '0', true
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10, true
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10, true
		default:
			return 0, false
		}
	}
	hi, okHigh := value(high)
	lo, okLow := value(low)
	return hi<<4 | lo, okHigh && okLow
}

func isPDFDecoderName(name string) bool {
	switch name {
	case "ASCII85Decode", "A85", "ASCIIHexDecode", "AHx", "RunLengthDecode", "RL",
		"LZWDecode", "LZW", "FlateDecode", "Fl", "CCITTFaxDecode", "CCF",
		"DCTDecode", "DCT", "JBIG2Decode", "JPXDecode", "Crypt":
		return true
	default:
		return false
	}
}

func hasIndirectPDFFilter(data []byte, valueStart int) bool {
	valueStart = skipPDFSpaceAndComments(data, valueStart)
	if valueStart >= len(data) {
		return false
	}
	if isPDFIndirectReferenceAt(data, valueStart) {
		return true
	}
	if data[valueStart] != '[' {
		return false
	}
	depth := 1
	for i := valueStart + 1; i < len(data); i++ {
		switch data[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				for j := valueStart + 1; j < i; j++ {
					if isPDFIndirectReferenceAt(data, j) {
						return true
					}
				}
				return false
			}
		}
	}
	return false
}

func isPDFIndirectReferenceAt(data []byte, start int) bool {
	firstEnd, ok := scanPDFInteger(data, start)
	if !ok {
		return false
	}
	secondStart := skipPDFSpaceAndComments(data, firstEnd)
	secondEnd, ok := scanPDFInteger(data, secondStart)
	if !ok {
		return false
	}
	rMarker := skipPDFSpaceAndComments(data, secondEnd)
	return rMarker < len(data) && data[rMarker] == 'R' &&
		(rMarker+1 == len(data) || isPDFDelimiter(data[rMarker+1]))
}

func scanPDFInteger(data []byte, start int) (int, bool) {
	if start >= len(data) || data[start] < '0' || data[start] > '9' {
		return start, false
	}
	end := start + 1
	for end < len(data) && data[end] >= '0' && data[end] <= '9' {
		end++
	}
	if end < len(data) && !isPDFDelimiter(data[end]) {
		return start, false
	}
	return end, true
}

func skipPDFSpaceAndComments(data []byte, start int) int {
	for start < len(data) {
		switch data[start] {
		case 0, '	', '\n', '\f', '\r', ' ':
			start++
		case '%':
			for start < len(data) && data[start] != '\n' && data[start] != '\r' {
				start++
			}
		default:
			return start
		}
	}
	return start
}

func countPDFStreamMarkers(data []byte) int64 {
	const keyword = "stream"
	var count int64
	for start := 0; start < len(data); {
		relative := bytes.Index(data[start:], []byte(keyword))
		if relative < 0 {
			break
		}
		at := start + relative
		after := at + len(keyword)
		if (at == 0 || isPDFWhitespace(data[at-1])) && after < len(data) &&
			(data[after] == '\n' || data[after] == '\r') {
			count++
		}
		start = after
	}
	return count
}

func isPDFDelimiter(b byte) bool {
	return isPDFWhitespace(b) || strings.ContainsRune("()<>[]{}/%", rune(b))
}

func isPDFWhitespace(b byte) bool {
	switch b {
	case 0, '	', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func reserveDecodedPDFBytes(used, requested, limit int64) (int64, bool) {
	if used < 0 || requested < 0 || limit < 0 || used > limit || requested > limit-used {
		return used, false
	}
	return used + requested, true
}

func (s *pdfStructureScanner) accountObjectStream(objectStreamNumber int) error {
	if s.ctx != nil && s.ctx.Err() != nil {
		return ErrRejected
	}
	if s.xref == nil || s.decodedBytes < 0 || objectStreamNumber <= 0 {
		return ErrRejected
	}
	if s.decodedObjectStreams == nil {
		s.decodedObjectStreams = make(map[int]struct{})
	}
	if _, seen := s.decodedObjectStreams[objectStreamNumber]; seen {
		return nil
	}
	if len(s.decodedObjectStreams) >= maxPDFObjectStreams {
		return ErrRejected
	}
	entry, found := s.xref.Find(objectStreamNumber)
	if !found || entry == nil || entry.Free {
		return ErrRejected
	}
	var objectStream types.ObjectStreamDict
	switch stream := entry.Object.(type) {
	case types.ObjectStreamDict:
		objectStream = stream
	case *types.ObjectStreamDict:
		if stream == nil {
			return ErrRejected
		}
		objectStream = *stream
	}
	if objectStream.FirstObjOffset < 0 || int64(objectStream.FirstObjOffset) > maxPDFObjectProlog ||
		objectStream.ObjCount < 1 || objectStream.ObjCount > maxPDFObjectStreams || len(objectStream.ObjArray) != objectStream.ObjCount {
		return ErrRejected
	}
	for _, object := range objectStream.ObjArray {
		switch lazy := object.(type) {
		case types.LazyObjectStreamObject:
		case *types.LazyObjectStreamObject:
			if lazy == nil {
				return ErrRejected
			}
		default:
			return ErrRejected
		}
	}
	limit := s.decodedByteLimit
	if limit <= 0 {
		limit = maxPDFDecodedTotal
	}
	if s.decodedBytes > limit {
		return ErrRejected
	}
	remaining := limit - s.decodedBytes
	streamLimit := maxPDFDecodedStream
	if objectStream.MaxDecodeBytes > 0 && objectStream.MaxDecodeBytes < streamLimit {
		streamLimit = objectStream.MaxDecodeBytes
	}
	if s.decoderStageLimit > 0 && s.decoderStageLimit < streamLimit {
		streamLimit = s.decoderStageLimit
	}
	if remaining < streamLimit {
		streamLimit = remaining
	}
	if streamLimit <= 0 || int64(objectStream.FirstObjOffset) > streamLimit {
		return ErrRejected
	}
	if objectStream.Content == nil {
		reserved, ok := reserveDecodedPDFBytes(s.decodedBytes, streamLimit, limit)
		if !ok {
			return ErrRejected
		}
		s.decodedBytes = reserved
		if err := objectStream.DecodeWithLimit(streamLimit); err != nil {
			return ErrRejected
		}
		if int64(len(objectStream.Content)) > streamLimit {
			return ErrRejected
		}
		s.decodedBytes = reserved - streamLimit + int64(len(objectStream.Content))
	} else {
		used, ok := reserveDecodedPDFBytes(s.decodedBytes, int64(len(objectStream.Content)), limit)
		if !ok || int64(len(objectStream.Content)) > streamLimit {
			return ErrRejected
		}
		s.decodedBytes = used
	}
	if s.ctx != nil && s.ctx.Err() != nil || objectStream.Content == nil || int64(len(objectStream.Content)) > streamLimit ||
		objectStream.FirstObjOffset > len(objectStream.Content) {
		return ErrRejected
	}

	prolog := append([]byte(nil), objectStream.Content[:objectStream.FirstObjOffset]...)
	if comment := bytes.IndexByte(prolog, '%'); comment >= 0 {
		prolog = prolog[:comment]
	}
	prolog = bytes.ReplaceAll(prolog, []byte{0}, []byte{' '})
	fields := strings.Fields(string(prolog))
	if len(fields) != objectStream.ObjCount*2 {
		return ErrRejected
	}

	objectNumbers := make([]int, objectStream.ObjCount)
	offsets := make([]int, objectStream.ObjCount)
	seenObjects := make(map[int]struct{}, objectStream.ObjCount)
	bodyLength := len(objectStream.Content) - objectStream.FirstObjOffset
	for index := 0; index < objectStream.ObjCount; index++ {
		objectNumber, err := strconv.Atoi(fields[index*2])
		if err != nil || objectNumber <= 0 {
			return ErrRejected
		}
		offset, err := strconv.Atoi(fields[index*2+1])
		if err != nil || offset < 0 || offset > bodyLength || index > 0 && offset < offsets[index-1] {
			return ErrRejected
		}
		if _, duplicate := seenObjects[objectNumber]; duplicate {
			return ErrRejected
		}
		seenObjects[objectNumber] = struct{}{}
		objectNumbers[index], offsets[index] = objectNumber, offset
	}
	for index, objectNumber := range objectNumbers {
		xrefEntry, found := s.xref.Find(objectNumber)
		if !found || xrefEntry == nil || xrefEntry.Free || xrefEntry.ObjectStream == nil ||
			*xrefEntry.ObjectStream != objectStreamNumber || xrefEntry.ObjectStreamInd == nil || *xrefEntry.ObjectStreamInd != index {
			return ErrRejected
		}
	}

	parent := s.ctx
	if parent == nil {
		parent = context.Background()
	}
	parseCtx, cancel := context.WithTimeout(parent, pdfParseTimeout)
	defer cancel()
	objects := make([]types.Object, objectStream.ObjCount)
	for index := range objects {
		if parseCtx.Err() != nil {
			return ErrRejected
		}
		start := objectStream.FirstObjOffset + offsets[index]
		end := len(objectStream.Content)
		if index+1 < len(objects) {
			end = objectStream.FirstObjOffset + offsets[index+1]
		}
		if start < objectStream.FirstObjOffset || end < start || end > len(objectStream.Content) {
			return ErrRejected
		}
		serialized := string(objectStream.Content[start:end])
		parsed, err := model.ParseObjectWithPolicy(parseCtx, &serialized, 0, model.ValidationStrict, maxPDFRecursion)
		if err != nil || strings.TrimSpace(serialized) != "" {
			return ErrRejected
		}
		if dictionary, ok := parsed.Object.(types.Dict); ok {
			length, lengthRef := dictionary.Length()
			if length != nil || lengthRef != nil {
				return ErrRejected
			}
		}
		objects[index] = parsed.Object
	}
	objectStream.ObjArray = types.Array(objects)
	entry.Object = objectStream
	s.decodedObjectStreams[objectStreamNumber] = struct{}{}
	if s.objectStreams == nil {
		s.objectStreams = make(map[int]decodedPDFObjectStream)
	}
	s.objectStreams[objectStreamNumber] = decodedPDFObjectStream{objectNumbers: objectNumbers, objects: objects}
	return nil
}

func (s *pdfStructureScanner) resolveReference(ref types.IndirectRef) (types.Object, error) {
	entry, found := s.xref.FindTableEntryForIndRef(&ref)
	if !found || entry == nil || entry.Free {
		return nil, ErrRejected
	}
	if entry.ObjectStream != nil {
		if entry.ObjectStreamInd == nil || int64(ref.GenerationNumber) != 0 {
			return nil, ErrRejected
		}
		streamNumber := *entry.ObjectStream
		if err := s.accountObjectStream(streamNumber); err != nil {
			return nil, err
		}
		stream, ok := s.objectStreams[streamNumber]
		index := *entry.ObjectStreamInd
		if !ok || index < 0 || index >= len(stream.objects) || stream.objectNumbers[index] != int(ref.ObjectNumber) {
			return nil, ErrRejected
		}
		return stream.objects[index], nil
	}
	if entry.Object == nil {
		return nil, ErrRejected
	}
	switch entry.Object.(type) {
	case types.LazyObjectStreamObject, *types.LazyObjectStreamObject:
		return nil, ErrRejected
	}
	return entry.Object, nil
}

func (s *pdfStructureScanner) walk(object types.Object, depth int) error {
	if s.ctx != nil && s.ctx.Err() != nil {
		return ErrRejected
	}
	if depth > maxPDFRecursion || s.walkNodes >= maxPDFStructureNodes {
		return ErrRejected
	}
	s.walkNodes++

	switch value := object.(type) {
	case nil:
		return nil
	case types.IndirectRef:
		ref := pdfObjectRef{objectNumber: int64(value.ObjectNumber), generationNumber: int64(value.GenerationNumber)}
		if _, ok := s.visited[ref]; ok {
			return nil
		}
		s.visited[ref] = struct{}{}
		resolved, err := s.resolveReference(value)
		if err != nil {
			return ErrRejected
		}
		return s.walk(resolved, depth+1)
	case types.Dict:
		return s.walkDict(value, depth)
	case types.Array:
		for _, element := range value {
			if err := s.walk(element, depth+1); err != nil {
				return err
			}
		}
	case types.StreamDict:
		return s.walkDict(value.Dict, depth+1)
	case *types.StreamDict:
		if value == nil {
			return ErrRejected
		}
		return s.walkDict(value.Dict, depth+1)
	case types.ObjectStreamDict:
		return s.walkDict(value.Dict, depth+1)
	case *types.ObjectStreamDict:
		if value == nil {
			return ErrRejected
		}
		return s.walkDict(value.Dict, depth+1)
	case types.XRefStreamDict:
		return s.walkDict(value.Dict, depth+1)
	case *types.XRefStreamDict:
		if value == nil {
			return ErrRejected
		}
		return s.walkDict(value.Dict, depth+1)
	case types.LazyObjectStreamObject:
		return ErrRejected
	case *types.LazyObjectStreamObject:
		if value == nil {
			return ErrRejected
		}
		return ErrRejected
	case types.Boolean, types.Float, types.Integer, types.Name, types.StringLiteral, types.HexLiteral:
		return nil
	default:
		return ErrRejected
	}
	return nil
}

func (s *pdfStructureScanner) walkDict(dict types.Dict, depth int) error {
	for _, key := range forbiddenPDFKeys {
		if _, found := dict.Find(key); found {
			return errPDFActiveContent
		}
	}

	for _, key := range []string{"Type", "Subtype", "S"} {
		name, found, err := s.nameEntry(dict, key)
		if err != nil {
			return ErrRejected
		}
		if !found {
			continue
		}
		switch key {
		case "Type":
			if name == "Filespec" || name == "FileSpec" || name == "EmbeddedFile" {
				return errPDFActiveContent
			}
		case "Subtype":
			if name == "Widget" || name == "RichMedia" || name == "3D" || name == "Movie" || name == "Sound" {
				return errPDFActiveContent
			}
		case "S":
			if _, forbidden := forbiddenPDFActions[name]; forbidden {
				return errPDFActiveContent
			}
		}
	}

	for _, value := range dict {
		if err := s.walk(value, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *pdfStructureScanner) nameEntry(dict types.Dict, key string) (string, bool, error) {
	object, found := dict.Find(key)
	if !found {
		return "", false, nil
	}
	if object == nil {
		return "", false, ErrRejected
	}
	resolved, err := s.dereference(object)
	if err != nil {
		return "", false, err
	}
	if resolved == nil {
		return "", false, ErrRejected
	}
	name, ok := resolved.(types.Name)
	if !ok {
		return "", false, ErrRejected
	}
	return string(name), true, nil
}

func (s *pdfStructureScanner) dereference(object types.Object) (types.Object, error) {
	ref, ok := object.(types.IndirectRef)
	if !ok {
		return object, nil
	}
	return s.resolveReference(ref)
}
