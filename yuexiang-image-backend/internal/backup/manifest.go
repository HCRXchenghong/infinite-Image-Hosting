package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Manifest struct {
	Version         string       `json:"version"`
	ExportedAt      time.Time    `json:"exported_at"`
	App             string       `json:"app"`
	Users           int          `json:"users"`
	Images          int          `json:"images"`
	Objects         []ObjectInfo `json:"objects"`
	IncludesSecrets bool         `json:"includes_secrets"`
	ChecksumFile    string       `json:"checksum_file"`
}

type ObjectInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func NewManifest(users, images int) Manifest {
	return Manifest{
		Version:         "2026-05-08",
		ExportedAt:      time.Now().UTC(),
		App:             "yuexiang-image",
		Users:           users,
		Images:          images,
		IncludesSecrets: false,
		ChecksumFile:    "checksums.sha256",
	}
}

func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
