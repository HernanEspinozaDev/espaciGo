// Package inspection validates and normalizes uploaded identity/space documents.
package inspection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"image"
	_ "image/gif"
	jpeg "image/jpeg"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"golang.org/x/sys/unix"
)

const maxBytes = 10 * 1024 * 1024

var ErrRejected = errors.New("document inspection rejected input")

type Config struct {
	MaxPixels                  int64
	PDFInfoPath, PDFDetachPath string
}
type Inspector struct {
	cfg                Config
	pdfInfo, pdfDetach *os.File
}

// New accepts only fixed Poppler binaries owned by root and stored under
// root-owned, non-writable directories. Never derive these paths from requests
// or tenant-controlled configuration.
func New(c Config) *Inspector {
	if c.MaxPixels <= 0 {
		c.MaxPixels = 40_000_000
	}
	pdfInfo, ok := openTrustedPDFExecutable(c.PDFInfoPath, "pdfinfo")
	if !ok {
		return nil
	}
	pdfDetach, ok := openTrustedPDFExecutable(c.PDFDetachPath, "pdfdetach")
	if !ok {
		pdfInfo.Close()
		return nil
	}
	if !trustedPDFExecutable(c.PDFInfoPath, "pdfinfo") || !trustedPDFExecutable(c.PDFDetachPath, "pdfdetach") {
		pdfInfo.Close()
		pdfDetach.Close()
		return nil
	}
	return &Inspector{cfg: c, pdfInfo: pdfInfo, pdfDetach: pdfDetach}
}

func openTrustedPDFExecutable(path, expectedName string) (*os.File, bool) {
	if !trustedPDFExecutable(path, expectedName) {
		return nil, false
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) || !trustedPDFExecutable(path, expectedName) {
		f.Close()
		return nil, false
	}
	return f, true
}

func runPinnedExecutable(ctx context.Context, file *os.File, args []string, stdin io.Reader, extraFiles []*os.File, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, "/proc/self/fd/3", args...)
	cmd.ExtraFiles = append([]*os.File{file}, extraFiles...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func trustedPDFExecutable(path, expectedName string) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) != expectedName {
		return false
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Mode().Perm()&0111 == 0 || info.Mode().Perm()&0022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || !trustedPDFDirectories(filepath.Dir(path)) {
		return false
	}
	return true
}

func trustedPDFDirectories(path string) bool {
	for {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
			return false
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != 0 {
			return false
		}
		parent := filepath.Dir(path)
		if parent == path {
			return true
		}
		path = parent
	}
}

// Inspect returns normalized, safe content and metadata derived only from that content.
func (i *Inspector) Inspect(p dv.Purpose, id string, data []byte) (dv.File, []byte, error) {
	reject := func() (dv.File, []byte, error) { return dv.File{}, nil, ErrRejected }
	if i == nil || (p != dv.PurposeIdentity && p != dv.PurposeSpace) || strings.TrimSpace(id) == "" || len(id) > 256 || len(data) == 0 || len(data) > maxBytes {
		return reject()
	}
	mime := ""
	pages := 1
	var normalized []byte
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		if !filepath.IsAbs(i.cfg.PDFInfoPath) || !filepath.IsAbs(i.cfg.PDFDetachPath) {
			return reject()
		}
		var normalizedData []byte
		var err error
		pages, normalizedData, err = i.inspectPDF(p, data)
		if err != nil {
			return reject()
		}
		mime = "application/pdf"
		normalized = normalizedData
	} else {
		_, typ, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || typ != "jpeg" && typ != "png" {
			return reject()
		}
		if typ == "png" && bytes.Contains(data, []byte("acTL")) {
			return reject()
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width) > i.cfg.MaxPixels/int64(cfg.Height) {
			return reject()
		}
		img, format, err := image.Decode(bytes.NewReader(data))
		if err != nil || format != typ {
			return reject()
		}
		if typ == "jpeg" && jpegOrientation(data) != 1 {
			return reject()
		}
		var b bytes.Buffer
		if typ == "png" {
			err = png.Encode(&b, img)
			mime = "image/png"
		} else {
			err = jpeg.Encode(&b, img, &jpeg.Options{Quality: 90})
			mime = "image/jpeg"
		}
		if err != nil {
			return reject()
		}
		normalized = b.Bytes()
	}
	if len(normalized) == 0 || len(normalized) > maxBytes {
		return reject()
	}
	return dv.File{ID: id, State: dv.FilePending, Size: int64(len(normalized)), MIME: mime, Pages: pages, ContentSHA256: sha256.Sum256(normalized)}, normalized, nil
}
func (i *Inspector) inspectPDF(p dv.Purpose, data []byte) (int, []byte, error) {
	runPDFInfo := func(args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), pdfCommandTimeout)
		defer cancel()
		var out boundedOutput
		var stderr boundedOutput
		if err := runPinnedExecutable(ctx, i.pdfInfo, args, bytes.NewReader(data), nil, &out, &stderr); err != nil || len(bytes.TrimSpace([]byte(stderr.String()))) != 0 {
			return "", ErrRejected
		}
		return out.String(), nil
	}
	info, err := runPDFInfo("-")
	if err != nil {
		return 0, nil, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		if a, b, ok := strings.Cut(line, ":"); ok {
			fields[strings.TrimSpace(a)] = strings.TrimSpace(b)
		}
	}
	for _, field := range []string{"Pages", "Encrypted", "JavaScript", "Form"} {
		if _, ok := fields[field]; !ok {
			return 0, nil, ErrRejected
		}
	}
	n, err := strconv.Atoi(fields["Pages"])
	limit := 5
	if p == dv.PurposeSpace {
		limit = 20
	}
	if err != nil || n < 1 || n > limit || !strings.EqualFold(fields["Encrypted"], "no") ||
		!strings.EqualFold(fields["JavaScript"], "no") || !strings.EqualFold(fields["Form"], "none") {
		return 0, nil, ErrRejected
	}
	if err := validatePDFStructure(data, i.cfg.MaxPixels, n); err != nil {
		return 0, nil, ErrRejected
	}
	js, err := runPDFInfo("-js", "-")
	if err != nil || strings.TrimSpace(js) != "" {
		return 0, nil, ErrRejected
	}
	urls, err := runPDFInfo("-url", "-")
	if err != nil || !hasEmptyPDFInfoURLList(urls) {
		return 0, nil, ErrRejected
	}
	fd, err := unix.MemfdCreate("document-inspection", unix.MFD_CLOEXEC)
	if err != nil {
		return 0, nil, err
	}
	f := os.NewFile(uintptr(fd), "memfd")
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return 0, nil, err
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return 0, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdfCommandTimeout)
	defer cancel()
	var out boundedOutput
	var stderr boundedOutput
	if err := runPinnedExecutable(ctx, i.pdfDetach, []string{"-list", "/proc/self/fd/4"}, nil, []*os.File{f}, &out, &stderr); err != nil || len(bytes.TrimSpace([]byte(stderr.String()))) != 0 {
		return 0, nil, ErrRejected
	}
	if !hasZeroEmbeddedPDFFiles(out.String()) {
		return 0, nil, ErrRejected
	}
	return n, append([]byte(nil), data...), nil
}

func hasZeroEmbeddedPDFFiles(output string) bool {
	fields := strings.Fields(output)
	return len(fields) == 3 && fields[0] == "0" &&
		strings.EqualFold(fields[1], "embedded") && strings.EqualFold(fields[2], "files")
}

func hasEmptyPDFInfoURLList(output string) bool {
	lines := strings.Split(output, "\n")
	header := false
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if !header {
			headerFields := map[string]bool{}
			for _, field := range fields {
				headerFields[strings.ToLower(field)] = true
			}
			if !headerFields["page"] || !headerFields["type"] || !headerFields["url"] {
				return false
			}
			header = true
			continue
		}
		return false
	}
	return header
}

func jpegOrientation(b []byte) int { // Conservatively reject EXIF-bearing JPEGs unless orientation is explicitly 1.
	for at := 2; at+4 < len(b); {
		if b[at] != 0xff {
			break
		}
		marker := b[at+1]
		at += 2
		if marker == 0xd9 || marker == 0xda {
			break
		}
		if at+2 > len(b) {
			break
		}
		size := int(b[at])<<8 | int(b[at+1])
		if size < 2 || at+size > len(b) {
			break
		}
		if marker == 0xe1 && bytes.Contains(b[at:at+size], []byte("Exif\x00\x00")) {
			return 0
		}
		at += size
	}
	return 1
}

var _ = image.ErrFormat
