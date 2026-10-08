package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const photoTestToken = "123456:secret-photo-token"

// fakePhotoAPI answers getUserProfilePhotos, getFile and the file download
// the way api.telegram.org does, recording what was asked for.
type fakePhotoAPI struct {
	photos      string // raw JSON for result.photos
	filePath    string
	fileSize    int64
	body        []byte
	contentType string
	status      int

	gotFileID   string
	gotLimit    float64
	downloaded  bool
	gotDownload string
}

func (f *fakePhotoAPI) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot" + photoTestToken + "/getUserProfilePhotos":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.gotLimit, _ = body["limit"].(float64)
			_, _ = w.Write([]byte(`{"ok":true,"result":{"total_count":1,"photos":` + f.photos + `}}`))
		case "/bot" + photoTestToken + "/getFile":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.gotFileID, _ = body["file_id"].(string)
			res, _ := json.Marshal(map[string]any{"file_id": f.gotFileID, "file_path": f.filePath, "file_size": f.fileSize})
			_, _ = w.Write([]byte(`{"ok":true,"result":` + string(res) + `}`))
		default:
			if !strings.HasPrefix(r.URL.Path, "/file/bot"+photoTestToken+"/") {
				http.NotFound(w, r)
				return
			}
			f.downloaded = true
			f.gotDownload = strings.TrimPrefix(r.URL.Path, "/file/bot"+photoTestToken+"/")
			if f.contentType != "" {
				w.Header().Set("Content-Type", f.contentType)
			}
			if f.status != 0 {
				w.WriteHeader(f.status)
			}
			_, _ = w.Write(f.body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const threeSizes = `[[
	{"file_id":"small","file_unique_id":"a","width":160,"height":160,"file_size":5000},
	{"file_id":"mid","file_unique_id":"b","width":320,"height":320,"file_size":20000},
	{"file_id":"big","file_unique_id":"c","width":640,"height":640,"file_size":60000}
]]`

func TestProfilePhotoPicksSizeClosestToTargetAndDownloads(t *testing.T) {
	f := &fakePhotoAPI{photos: threeSizes, filePath: "photos/file_7.jpg", body: []byte("JPEGBYTES"), contentType: "image/jpeg"}
	srv := f.server(t)
	c := NewClient(srv.URL, photoTestToken, srv.Client())

	data, ct, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20)
	if err != nil {
		t.Fatalf("ProfilePhoto: %v", err)
	}
	if f.gotLimit != 1 {
		t.Errorf("limit = %v, want 1 (only the current photo)", f.gotLimit)
	}
	if f.gotFileID != "mid" {
		t.Errorf("file_id = %q, want the 320px size", f.gotFileID)
	}
	if f.gotDownload != "photos/file_7.jpg" {
		t.Errorf("downloaded %q", f.gotDownload)
	}
	if string(data) != "JPEGBYTES" || ct != "image/jpeg" {
		t.Fatalf("data=%q ct=%q", data, ct)
	}
}

func TestProfilePhotoNoPhotosIsNotAnError(t *testing.T) {
	f := &fakePhotoAPI{photos: `[]`}
	srv := f.server(t)
	c := NewClient(srv.URL, photoTestToken, srv.Client())

	data, _, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20)
	if err != nil || data != nil {
		t.Fatalf("data=%v err=%v, want nil,nil for a hidden or absent photo", data, err)
	}
	if f.downloaded {
		t.Fatal("downloaded a file although there were no photos")
	}
}

func TestProfilePhotoRefusesOversizeBeforeAndDuringDownload(t *testing.T) {
	// file_size announced by getFile already over the cap: no download at all.
	f := &fakePhotoAPI{photos: threeSizes, filePath: "photos/x.jpg", fileSize: 3 << 20, body: []byte("x"), contentType: "image/jpeg"}
	srv := f.server(t)
	c := NewClient(srv.URL, photoTestToken, srv.Client())
	if _, _, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20); !errors.Is(err, ErrPhotoTooLarge) {
		t.Fatalf("err=%v, want ErrPhotoTooLarge", err)
	}
	if f.downloaded {
		t.Fatal("downloaded a file getFile said was too large")
	}

	// file_size absent, the body itself is too large.
	f2 := &fakePhotoAPI{photos: threeSizes, filePath: "photos/y.jpg", body: bytes.Repeat([]byte{0xff}, 1025), contentType: "image/jpeg"}
	srv2 := f2.server(t)
	c2 := NewClient(srv2.URL, photoTestToken, srv2.Client())
	if _, _, err := c2.ProfilePhoto(context.Background(), 42, 320, 1024); !errors.Is(err, ErrPhotoTooLarge) {
		t.Fatalf("err=%v, want ErrPhotoTooLarge", err)
	}
}

func TestProfilePhotoRejectsNonImageContentType(t *testing.T) {
	f := &fakePhotoAPI{photos: threeSizes, filePath: "photos/z.jpg", body: []byte("<html>"), contentType: "text/html"}
	srv := f.server(t)
	c := NewClient(srv.URL, photoTestToken, srv.Client())
	if _, _, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20); !errors.Is(err, ErrPhotoNotImage) {
		t.Fatalf("err=%v, want ErrPhotoNotImage", err)
	}
}

func TestProfilePhotoRejectsSuspiciousFilePath(t *testing.T) {
	for _, p := range []string{"../bot123/getMe", "photos/../../x", "/etc/passwd", "photos/a.jpg?x=1", "photos/a.jpg#f", ""} {
		f := &fakePhotoAPI{photos: threeSizes, filePath: p, body: []byte("x"), contentType: "image/jpeg"}
		srv := f.server(t)
		c := NewClient(srv.URL, photoTestToken, srv.Client())
		if _, _, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20); err == nil {
			t.Errorf("file_path %q accepted", p)
		}
		if f.downloaded {
			t.Errorf("file_path %q was downloaded", p)
		}
	}
}

// The download URL carries the bot token in its path, exactly like the API
// URL: neither an HTTP failure nor a transport failure may echo it.
func TestProfilePhotoErrorsNeverContainToken(t *testing.T) {
	f := &fakePhotoAPI{photos: threeSizes, filePath: "photos/gone.jpg", status: http.StatusNotFound, contentType: "text/plain", body: []byte("Not Found")}
	srv := f.server(t)
	c := NewClient(srv.URL, photoTestToken, srv.Client())
	_, _, err := c.ProfilePhoto(context.Background(), 42, 320, 2<<20)
	if err == nil {
		t.Fatal("want error on HTTP 404")
	}
	if strings.Contains(err.Error(), photoTestToken) || strings.Contains(err.Error(), "secret-photo-token") {
		t.Fatalf("token leaked: %q", err)
	}
	if strings.Contains(err.Error(), "photos/gone.jpg") {
		t.Fatalf("file path leaked: %q", err)
	}

	// Transport failure on the download itself: the API calls succeed, then
	// the file host closes the connection.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUserProfilePhotos"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"total_count":1,"photos":` + threeSizes + `}}`))
		case strings.HasSuffix(r.URL.Path, "/getFile"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"file_id":"mid","file_path":"photos/p.jpg"}}`))
		default:
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Error("no hijacker")
				return
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer api.Close()
	c = NewClient(api.URL, photoTestToken, api.Client())
	_, _, err = c.ProfilePhoto(context.Background(), 42, 320, 2<<20)
	if err == nil {
		t.Fatal("want transport error")
	}
	if strings.Contains(err.Error(), "secret-photo-token") || strings.Contains(err.Error(), "/file/bot") {
		t.Fatalf("token or file URL leaked: %q", err)
	}
}
