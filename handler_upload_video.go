package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading video", videoID, "by user", userID)

	const maxMemory = 1 << 30
	limitedBody := http.MaxBytesReader(w, r.Body, maxMemory)
	r.Body = limitedBody

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		panic(err)
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, 500, "Error reading the database", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, 401, "Unauthorized", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	mediatype, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		panic(err)
	}

	if mediatype != "video/mp4" {
		respondWithError(w, 415, "Unsupported media type", err)
		return
	}

	f, err := os.CreateTemp("", "tubelyl-upload.mp4")
	if err != nil {
		respondWithError(w, 500, "Error while creating temp file", err)
		return
	}
	defer os.Remove(f.Name())
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		panic(err)
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		panic(err)
	}

	aspectRatio, err := getVideoAspectRatio(f.Name())
	if err != nil {
		respondWithError(w, 500, "Can't read aspect ratio", err)
		return
	}

	var layout string

	switch aspectRatio {
	case "16:9":
		layout = "landscape"
	case "9:16":
		layout = "portrait"
	default:
		layout = "other"
	}

	key := make([]byte, 32)
	_, err = rand.Read(key)
	if err != nil {
		respondWithError(w, 500, "Couldn't generate key", err)
		return
	}

	fileKey := fmt.Sprintf("%s/%x.mp4", layout, key)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileKey),
		Body:        f,
		ContentType: aws.String(mediaType),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), input)
	if err != nil {
		respondWithError(w, 500, "Couldn't upload video to S3", err)
		return
	}

	VideoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)

	video.VideoURL = &VideoURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
