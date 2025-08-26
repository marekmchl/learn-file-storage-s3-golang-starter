package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const uploadLimit int64 = 10 << 30 // == 10 * 1024^3 == 1GB
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)

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

	video, err := cfg.db.GetVideo(videoID)
	// respondWithError(w, http.StatusInternalServerError, "Get video failed", err)
	if err != nil && video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized action by user", err)
		return
	}

	videoFile, fileHeaders, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "FormFile failed", err)
		return
	}
	defer videoFile.Close()

	tempLocalVideoFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "CreateTemp failed", err)
		return
	}
	defer os.Remove(tempLocalVideoFile.Name())
	defer tempLocalVideoFile.Close()

	_, err = io.Copy(tempLocalVideoFile, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Copy failed", err)
		return
	}

	_, err = tempLocalVideoFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Seek failed", err)
		return
	}

	mediaType, _, err := mime.ParseMediaType(fileHeaders.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed parsing content type", err)
		return
	}

	mediaTypeSlice := strings.Split(mediaType, "/")
	if len(mediaTypeSlice) < 2 || mediaTypeSlice[0] != "video" {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
		return
	}
	videoExtension := mediaTypeSlice[1]

	videoNameBytes := make([]byte, 32)
	if _, err := rand.Read(videoNameBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed reading the uploaded file", err)
		return
	}

	videoName := fmt.Sprintf("%v.%v", base64.RawURLEncoding.EncodeToString(videoNameBytes), videoExtension)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &videoName,
		Body:        tempLocalVideoFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed PutObject", err)
		return
	}

	videoURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, videoName)
	fmt.Println(videoURL)

	video.UpdatedAt = time.Now()
	video.VideoURL = &videoURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Update video failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
