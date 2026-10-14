package api

import (
	"bytes"
	"genco/finance/internal/model"
	"genco/finance/internal/store"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func resize(src image.Image, max int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= max && h <= max {
		return src
	}
	if w >= h {
		h = h * max / w
		w = max
	} else {
		w = w * max / h
		h = max
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}
func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	if err := r.ParseMultipartForm(6 << 20); err != nil {
		return bad("Image limit is 5 MB")
	}
	defer r.MultipartForm.RemoveAll()
	v, err := version(r)
	if err != nil {
		return err
	}
	file, h, err := r.FormFile("file")
	if err != nil {
		return bad("Choose an image")
	}
	defer file.Close()
	if h.Size > 5<<20 {
		return bad("Image limit is 5 MB")
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return bad("Choose a JPEG, PNG, or WebP image")
	}
	if config.Width*config.Height > 25000000 || config.Width > 12000 || config.Height > 12000 {
		return bad("Image dimensions exceed 25 megapixels")
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return bad("Image could not be decoded")
	}
	id := store.ID()
	dir := filepath.Join(s.Dir, "images")
	if err = os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	for name, img := range map[string]image.Image{id: resize(src, 2400), id + "-thumb": resize(src, 320)} {
		path := filepath.Join(dir, name+".jpg")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if err != nil {
			return err
		}
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 88})
		ce := f.Close()
		if err != nil {
			return err
		}
		if ce != nil {
			return ce
		}
	}
	var p model.Product
	err = s.Store.Change(r.Context(), func(st *model.State) error {
		old, ok := st.Products[r.PathValue("id")]
		if !ok {
			return missing()
		}
		if old.Version != v {
			return versionConflict("This product has changed. Reload before uploading.")
		}
		if len(old.Images) >= 5 {
			return bad("Maximum five screenshots per product")
		}
		p = old
		p.Version++
		p.UpdatedAt = time.Now().UTC()
		p.Images = append(p.Images, model.Image{ID: id, Name: h.Filename, URL: "/api/v1/images/" + id, Thumbnail: "/api/v1/images/" + id + "-thumb", CreatedAt: time.Now().UTC()})
		st.Products[p.ID] = p
		return nil
	})
	if err != nil {
		_ = os.Remove(filepath.Join(dir, id+".jpg"))
		_ = os.Remove(filepath.Join(dir, id+"-thumb.jpg"))
		return err
	}
	respond(w, 201, p)
	return nil
}
func (s *Server) deleteImage(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Version int `json:"version"`
	}
	if err := decode(w, r, &req); err != nil {
		return err
	}
	id := r.PathValue("image")
	var p model.Product
	err := s.Store.Change(r.Context(), func(st *model.State) error {
		old, ok := st.Products[r.PathValue("id")]
		if !ok {
			return missing()
		}
		if old.Version != req.Version {
			return versionConflict("This product has changed. Reload before deleting.")
		}
		p = old
		imgs := []model.Image{}
		found := false
		for _, im := range old.Images {
			if im.ID == id {
				found = true
			} else {
				imgs = append(imgs, im)
			}
		}
		if !found {
			return missing()
		}
		p.Images = imgs
		p.Version++
		p.UpdatedAt = time.Now().UTC()
		st.Products[p.ID] = p
		return nil
	})
	if err != nil {
		return err
	}
	for _, name := range []string{id, id + "-thumb"} {
		_ = os.Remove(filepath.Join(s.Dir, "images", name+".jpg"))
	}
	respond(w, 200, p)
	return nil
}
func (s *Server) serveImage(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("image")
	id := strings.TrimSuffix(name, "-thumb")
	if !validID(id) {
		return missing()
	}
	found := false
	for _, p := range s.Store.Read().Products {
		for _, im := range p.Images {
			if im.ID == id {
				found = true
			}
		}
	}
	if !found {
		return missing()
	}
	path := filepath.Join(s.Dir, "images", name+".jpg")
	if _, err := os.Stat(path); err != nil {
		return missing()
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=300")
	http.ServeFile(w, r, path)
	return nil
}
