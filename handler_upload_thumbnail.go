package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	videoMeta, err := cfg.db.GetVideo(videoID)

	if videoMeta.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error loading the video metadata", err)
	}

	// newURL := base64.StdEncoding.EncodeToString(video)
	// data := fmt.Sprintf("data:%v;base64,%v", mediaType, newURL)
	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error createing URL prefix", err)
		return
	}

	URLPrefix := base64.RawURLEncoding.EncodeToString(key)
	path := URLPrefix + "." + strings.Split(mediaType, "/")[1]

	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, path)
	videoMeta.ThumbnailURL = &thumbnailURL

	newFile, err := os.Create(filepath.Join(cfg.assetsRoot, path))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error creating the file", err)
		return
	}

	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error while copying file", err)
		return
	}

	err = cfg.db.UpdateVideo(videoMeta)
	if err != nil {
		respondWithError(w, 500, "Error updating video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMeta)
}
