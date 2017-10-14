package meta

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	strftime "github.com/jehiah/go-strftime"
	"github.com/pkg/errors"
)

type Binary struct {
	Name      string `yaml:"name"`
	Checksum  string `yaml:"checksum"`
	Timestamp string `yaml:"timestamp"`
	Version   string `yaml:"version,omitempty"`
}

type Meta struct {
	Binaries []*Binary `yaml:"binaries"`
}

func New(b *Binary) *Meta {
	return &Meta{Binaries: []*Binary{b}}
}

func (m *Meta) AppendBinary(b *Binary) {
	m.Binaries = append(m.Binaries, b)
}

func BuildBinary(r io.Reader, name string) (*Binary, error) {
	sum, err := checksum(r)
	if err != nil {
		return nil, err
	}
	return &Binary{
		Name:      name,
		Checksum:  sum,
		Timestamp: now(),
	}, nil
}

func now() string {
	t := time.Now()
	utc, _ := time.LoadLocation("UTC")
	t = t.In(utc)
	return strftime.Format("%Y%m%d%H%M%S", t)
}

func checksum(r io.Reader) (string, error) {
	body, err := ioutil.ReadAll(r)
	if err != nil {
		errors.Errorf("failed to read data for checksum")
	}
	return fmt.Sprintf("%x", sha1.Sum(body)), nil
}