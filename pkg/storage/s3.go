package storage

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/pkg/errors"
	"github.com/yuuki/binrep/pkg/meta"
)

const (
	BIN_NAME       = "BINARY"
	META_FILE_NAME = "meta.yml"
)

type S3 interface {
	LatestTimestamp(urlStr string, name string) (string, error)
	CreateOrUpdateMeta(u *url.URL, bins []*meta.Binary) error
	PushBinary(in io.Reader, url *url.URL, binName string) (string, error)
	PullBinary(w io.WriterAt, url *url.URL, binName string) error
	PullBinaries(u *url.URL, installDir string) error
}

type _s3 struct {
	svc        s3iface.S3API
	uploader   s3manageriface.UploaderAPI
	downloader s3manageriface.DownloaderAPI
}

// BuildURL builds the binary file url for S3.
func BuildURL(urlStr string, name, timestamp string) (*url.URL, error) {
	//TODO: validate version
	u, err := url.Parse(urlStr + "/" + filepath.Join(name, timestamp))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %v", urlStr)
	}
	return u, nil
}

// New creates a S3 client object.
func New(sess *session.Session) S3 {
	return &_s3{
		svc:        s3.New(sess),
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
	}
}

// LatestTimestamp gets the latest timestamp.
func (s *_s3) LatestTimestamp(urlStr string, name string) (string, error) {
	u, err := url.Parse(urlStr + "/" + name)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse %v", urlStr)
	}
	resp, err := s.svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket:    aws.String(u.Host),
		Prefix:    aws.String(strings.TrimLeft(u.Path, "/") + "/"),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to list objects (bucket: %v, path: %v/)", u.Host, u.Path)
	}
	if len(resp.CommonPrefixes) < 1 {
		return "", errors.Errorf("no such projects %v", name)
	}
	timestamps := make([]string, 0, len(resp.CommonPrefixes))
	for _, cp := range resp.CommonPrefixes {
		timestamps = append(timestamps, filepath.Base(*cp.Prefix))
	}
	sort.Strings(timestamps)
	return timestamps[len(timestamps)-1], nil
}

func (s *_s3) CreateMeta(u *url.URL, bins []*meta.Binary) error {
	m := meta.New(bins)
	data, err := yaml.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "failed to marshal yaml")
	}
	_, err = s.svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(filepath.Join(u.Path, META_FILE_NAME)),
		Body:   aws.ReadSeekCloser(bytes.NewReader(data)),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to put meta.yml into s3 (%s)", u)
	}
	return nil
}

// FindMeta finds metadata from S3, and returns nil if meta.yml is not found.
func (s *_s3) FindMeta(u *url.URL) (*meta.Meta, error) {
	resp, err := s.svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(filepath.Join(u.Path, META_FILE_NAME)),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				return nil, nil
			default:
			}
		}
		return nil, errors.Wrapf(err, "failed to get object from s3 %s", u)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read meta.yml on s3")
	}
	var m meta.Meta
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, errors.Wrapf(err, "failed to read meta.yml on s3")
	}
	return &m, nil
}

func (s *_s3) CreateOrUpdateMeta(u *url.URL, bins []*meta.Binary) error {
	m, err := s.FindMeta(u)
	if err != nil {
		return err
	}
	if m == nil {
		if err := s.CreateMeta(u, bins); err != nil {
			return err
		}
		return nil
	}

	m.AppendBinaries(bins)
	data, err := yaml.Marshal(m)
	if err != nil {
		return errors.Wrapf(err, "failed to unmsarshal meta")
	}
	_, err = s.svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(filepath.Join(u.Path, META_FILE_NAME)),
		Body:   aws.ReadSeekCloser(bytes.NewBuffer(data)),
	})
	if err != nil {
		return errors.Wrap(err, "failed to put meta.yml into s3")
	}

	return nil
}

// PushBinary pushes the binary file data into S3.
func (s *_s3) PushBinary(in io.Reader, url *url.URL, binName string) (string, error) {
	result, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(url.Host),
		Key:    aws.String(filepath.Join(url.Path, binName)),
		Body:   in,
	})
	if err != nil {
		return "", errors.Wrapf(err, "failed to upload file to %s", url)
	}
	return result.Location, nil
}

// PullBinary pulls the binary file data from S3.
func (s *_s3) PullBinary(w io.WriterAt, u *url.URL, binName string) error {
	_, err := s.downloader.Download(w, &s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(filepath.Join(u.Path, binName)),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to upload file to %v", u)
	}
	return nil
}

func (s *_s3) PullBinaries(u *url.URL, installDir string) error {
	m, err := s.FindMeta(u)
	if err != nil {
		return err
	}
	if m == nil {
		return errors.Errorf("meta.yml not found %s", u)
	}
	for _, bin := range m.Binaries {
		path := filepath.Join(installDir, bin.Name)
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to open %v", path)
		}
		if err := s.PullBinary(file, u, bin.Name); err != nil {
			return err
		}
		if err := bin.ValidateChecksum(file); err != nil {
			os.Remove(path)
			return err
		}
	}
	return nil
}