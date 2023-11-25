package storage

import (
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	LocalDirectory string
}

type Storage struct {
	cfg Config
}

// NewStorage creates a new local file storage instance
func NewStorage(cfg Config) *Storage {
	return &Storage{cfg: cfg}
}

// GetPath returns a filename joined with the local storage directory
func (s *Storage) GetPath(filename string) string {
	return filepath.Join(s.cfg.LocalDirectory, filename)
}

// CheckFile checks is a file exists and if there are any errors accessing it
func (s *Storage) CheckFile(filename string) (exists bool, err error) {
	_, err = os.Stat(s.GetPath(filename))
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// WriteFile stores the data into a file in the local storage directory
func (s *Storage) WriteFile(filename string, data []byte) error {
	return os.WriteFile(s.GetPath(filename), data, 0666)
}

// ServeFile replies to the request with the contents of the file from the local storage directory
func (s *Storage) ServeFile(w http.ResponseWriter, r *http.Request, filename string) {
	http.ServeFile(w, r, s.GetPath(filename))
}

type FileProperties struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// GetFileList returns a list of properties of all files located in the local storage directory
func (s *Storage) GetFileList() ([]FileProperties, error) {
	files, err := os.ReadDir(s.cfg.LocalDirectory)
	if err != nil {
		return nil, err
	}

	out := []FileProperties{}
	for _, v := range files {
		props := FileProperties{
			Name: v.Name(),
		}

		fi, err := v.Info()
		if err == nil {
			props.Size = fi.Size()
		}

		out = append(out, props)
	}

	return out, nil
}
