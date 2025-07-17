package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

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

	// upload implemented here

	const maxMemory int64 = 10 << 20 // == 10 * 1024 * 1024 == 10MB

	if err := r.ParseMultipartForm(maxMemory); err != nil {
		respondWithError(w, http.StatusInternalServerError, "ParseMultipartForm failed", err)
		return
	}

	file, fileHeaders, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "FormFile failed", err)
		return
	}
	defer file.Close()

	mediaType := fileHeaders.Header.Get("Content-Type")
	imageBytes, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "File read failed", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Get video failed", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized action by user", err)
		return
	}

	videoThumbnails[videoID] = thumbnail{
		data:      imageBytes,
		mediaType: mediaType,
	}

	thumbURL := fmt.Sprintf("http://localhost:%v/api/thumbnails/{%v}", cfg.port, videoID)
	video.UpdatedAt = time.Now()
	video.ThumbnailURL = &thumbURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Update video failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
