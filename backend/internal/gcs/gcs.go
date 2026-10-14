// Package gcs wraps Google Cloud Storage so the SDK's details stay out of the
// API layer. Objects are served by streaming through the backend (not signed
// URLs), so this exposes Upload, Open (a streaming reader), Download (to a
// local path for tools that need a file), Stat, and Delete against a single
// bucket. Authentication is via a downloaded service-account JSON key, matching
// the deployment used elsewhere in Genco.
package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// ErrNotExist is returned when an object is not present in the bucket. It wraps
// storage.ErrObjectNotExist so callers can match either.
var ErrNotExist = storage.ErrObjectNotExist

// Store holds a GCS client bound to a single bucket and an optional key prefix.
type Store struct {
	client *storage.Client
	bucket string
	prefix string
}

// New builds a Store from a service-account key file. prefix (may be empty) is
// prepended to every object key, letting one bucket host multiple apps.
func New(ctx context.Context, bucket, credentialsFile, prefix string) (*Store, error) {
	if bucket == "" {
		return nil, errors.New("gcs: bucket is not set")
	}
	if credentialsFile == "" {
		return nil, errors.New("gcs: credentials file is not set")
	}
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("gcs: read credentials file: %w", err)
	}
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(raw))
	if err != nil {
		return nil, fmt.Errorf("gcs: new storage client: %w", err)
	}
	prefix = strings.Trim(prefix, "/")
	return &Store{client: client, bucket: bucket, prefix: prefix}, nil
}

// key applies the configured prefix to a logical object name.
func (s *Store) key(name string) string {
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

func (s *Store) object(name string) *storage.ObjectHandle {
	return s.client.Bucket(s.bucket).Object(s.key(name))
}

// Upload streams r to the named object with the given content type. The write
// is finalized on Close; any streaming error is surfaced there.
func (s *Store) Upload(ctx context.Context, name string, r io.Reader, contentType string) error {
	w := s.object(name).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := io.Copy(w, r); err != nil {
		_ = w.Close()
		return fmt.Errorf("gcs: upload %q: %w", name, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("gcs: finalize %q: %w", name, err)
	}
	return nil
}

// Open returns a streaming reader for the named object plus its content type.
// The caller must Close the reader. A missing object returns ErrNotExist.
func (s *Store) Open(ctx context.Context, name string) (io.ReadCloser, string, error) {
	r, err := s.object(name).NewReader(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("gcs: open %q: %w", name, err)
	}
	return r, r.Attrs.ContentType, nil
}

// Download writes the named object to a local path (created with perm). It is
// used by tools (e.g. the Excel reader) that need a real file. A missing object
// returns ErrNotExist.
func (s *Store) Download(ctx context.Context, name, path string, perm os.FileMode) error {
	r, err := s.object(name).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("gcs: download %q: %w", name, err)
	}
	defer r.Close()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, cpErr := io.Copy(f, r)
	closeErr := f.Close()
	if cpErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.Join(cpErr, closeErr)
	}
	return nil
}

// Created returns the object's creation time, used for age-based expiry. A
// missing object returns ErrNotExist.
func (s *Store) Created(ctx context.Context, name string) (time.Time, error) {
	attrs, err := s.object(name).Attrs(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("gcs: stat %q: %w", name, err)
	}
	return attrs.Created, nil
}

// Delete removes the named object. A missing object is treated as success so
// deletes are idempotent.
func (s *Store) Delete(ctx context.Context, name string) error {
	err := s.object(name).Delete(ctx)
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return fmt.Errorf("gcs: delete %q: %w", name, err)
	}
	return nil
}

// Close releases the underlying storage client.
func (s *Store) Close() error { return s.client.Close() }
