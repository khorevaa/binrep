package release

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
)

const (
	shortCheckSumLen int = 7
)

// Binary represents the binary file within release.
type Binary struct {
	Name     string    `yaml:"name"`
	Checksum string    `yaml:"checksum"`
	Version  string    `yaml:"version,omitempty"`
	Body     io.Reader `yaml:"-"`
}

// BuildBinary builds a Binary object. Return error if it is failed
// to calculate checksum of the body.
func BuildBinary(name string, body io.Reader) (*Binary, error) {
	sum, err := checksum(body)
	if err != nil {
		return nil, err
	}
	return &Binary{
		Name:     name,
		Checksum: sum,
		Body:     body,
	}, nil
}

func checksum(r io.Reader) (string, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		errors.Errorf("failed to read data for checksum")
	}
	return fmt.Sprintf("%x", sha256.Sum256(body)), nil
}

// ValidateChecksum validates the correctness of the checksum. Return
// error If the both of checksum is not the same.
func (b *Binary) ValidateChecksum(r io.Reader) error {
	sum, err := checksum(r)
	if err != nil {
		return err
	}
	if b.Checksum != sum {
		return errors.Errorf("invalid checksum, got %v, want %v", sum, b.Checksum)
	}
	return nil
}

func (b *Binary) shortChecksum() string {
	return b.Checksum[0:shortCheckSumLen]
}

// Read reads the Binary.Body.
func (b *Binary) Read(r io.Reader) ([]byte, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read binary body %v", b.Name)
	}
	return data, nil
}

// Inspect prints the binary information.
func (b *Binary) Inspect(w io.Writer) {
	fmt.Fprintf(w, "%s/%s/%s\t", b.Name, b.Version, b.shortChecksum())
}
