package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"videoservice/framer"
	"videoservice/storage"
)

type Config struct {
	IsProd bool

	ListenPort int

	ThumbnailWidth  int
	ThumbnailHeight int
}

type API struct {
	cfg Config
	frm *framer.Framer
	sto *storage.Storage
}

// NewAPI creates a new VideoAPI instance
func NewAPI(cfg Config, frm *framer.Framer, sto *storage.Storage) (*API, error) {
	if cfg.ListenPort <= 0 {
		cfg.ListenPort = 3000
	}
	if cfg.ThumbnailWidth <= 0 {
		cfg.ThumbnailWidth = 128
	}
	if cfg.ThumbnailHeight <= 0 {
		cfg.ThumbnailHeight = 128
	}
	return &API{
		cfg: cfg,
		frm: frm,
		sto: sto,
	}, nil
}

// GetVideo handles requests to play a video file
func (a *API) GetVideo(w http.ResponseWriter, r *http.Request) {
	filename := mux.Vars(r)["filename"]
	a.sto.ServeFile(w, r, filename)
}

// GetFrame handles requests to get a frame of a video file
func (a *API) GetFrame(w http.ResponseWriter, r *http.Request) {
	var err error

	width := 0
	str := r.URL.Query().Get("width")
	if str != "" {
		width, err = strconv.Atoi(str)
		if err != nil {
			http.Error(w, fmt.Sprintf("width parsing error: %v", err), http.StatusBadRequest)
			return
		}
	}

	height := 0
	str = r.URL.Query().Get("height")
	if str != "" {
		height, err = strconv.Atoi(str)
		if err != nil {
			http.Error(w, fmt.Sprintf("height parsing error: %v", err), http.StatusBadRequest)
			return
		}
	}

	a.handleGetFrameRequest(w, r, width, height, false)
}

// GetThumbnail handles requests to get a thumbnail of a frame of a video file
func (a *API) GetThumbnail(w http.ResponseWriter, r *http.Request) {
	a.handleGetFrameRequest(w, r, a.cfg.ThumbnailWidth, a.cfg.ThumbnailHeight, true)
}

// handleGetFrameRequest is a helper function to process requests to get a frame of a video file
func (a *API) handleGetFrameRequest(w http.ResponseWriter, r *http.Request, width int, height int, thumbnail bool) {
	filename := mux.Vars(r)["filename"]
	frameIndex, err := strconv.Atoi(mux.Vars(r)["index"])
	if err != nil {
		http.Error(w, fmt.Sprintf("frame index parsing error: %v", err), http.StatusBadRequest)
		return
	}

	ch := a.frm.EnqueueJob(r.Context(), a.sto.GetPath(filename), frameIndex, width, height, thumbnail)

	select {
	case <-r.Context().Done():
		http.Error(w, "context closed", http.StatusInternalServerError)
		return

	case res := <-ch:
		if res == nil {
			http.Error(w, "context closed", http.StatusInternalServerError)
			return
		}

		if res.Err != nil {
			http.Error(w, fmt.Sprintf("ffmpeg error: %v", res.Err), http.StatusInternalServerError)
			return
		}

		if len(res.Data) == 0 {
			http.Error(w, "frame index out of bounds", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeContent(w, r, filename, time.Now(), bytes.NewReader(res.Data))
	}
}

// GetVideos handles requests to get a list of video files
func (a *API) GetVideos(w http.ResponseWriter, r *http.Request) {

	files, err := a.sto.GetFileList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := struct {
		Files []storage.FileProperties `json:"files"`
	}{
		Files: files,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&resp)
}

// UploadVideo handles requests to upload a video file
func (a *API) UploadVideo(w http.ResponseWriter, r *http.Request) {

	err := r.ParseMultipartForm(32 << 20) // 32Mb
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	defer r.MultipartForm.RemoveAll()

	// Start reading multi-part file under id "filename"
	f, fh, err := r.FormFile("filename")
	if err != nil {
		if err == http.ErrMissingFile {
			http.Error(w, "request does not contain a filename", http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		return
	}
	defer f.Close()

	exists, err := a.sto.CheckFile(fh.Filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "file exists", http.StatusBadRequest)
		return
	}

	buf := bytes.Buffer{}
	_, err = buf.ReadFrom(f)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = a.sto.WriteFile(fh.Filename, buf.Bytes())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Run starts serving VideoAPI
func (a *API) Run(ctx context.Context) error {

	r := mux.NewRouter()

	r.HandleFunc("/video/{filename}", a.GetVideo).Methods("GET")
	r.HandleFunc("/video/{filename}/frame/{index}", a.GetFrame).Methods("GET")
	r.HandleFunc("/video/{filename}/frame/{index}/thumbnail", a.GetThumbnail).Methods("GET")
	r.HandleFunc("/videos", a.GetVideos).Methods("GET")
	r.HandleFunc("/upload", a.UploadVideo).Methods("POST")

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf(":%v", a.cfg.ListenPort),
		WriteTimeout: 150 * time.Second,
		ReadTimeout:  150 * time.Second,
	}

	go func() {
		<-ctx.Done()

		cctx, ccancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ccancel()

		srv.Shutdown(cctx)
	}()

	return srv.ListenAndServe()
}
