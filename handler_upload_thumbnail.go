package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse multipart form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	extensions, _ := mime.ExtensionsByType(mediaType)
	ext := ""
	if len(extensions) > 0 {
		ext = extensions[0]
	} else {
		ext = filepath.Ext(header.Filename)
	}
	if ext == "" {
		respondWithError(w, http.StatusBadRequest, "Unable to determine file extension", nil)
		return
	}

	randBytes := make([]byte, 32)
	if _, err := rand.Read(randBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to generate thumbnail file name", err)
		return
	}

	randomName := base64.RawURLEncoding.EncodeToString(randBytes)
	assetPath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%s%s", randomName, ext))
	dest, err := os.Create(assetPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create thumbnail file", err)
		return
	}
	defer dest.Close()

	if _, err := io.Copy(dest, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save thumbnail file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if video.ID == uuid.Nil {
		respondWithError(w, http.StatusNotFound, "Video not found", nil)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You are not allowed to update this video's thumbnail", nil)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, randomName, ext)
	video.ThumbnailURL = &thumbnailURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
