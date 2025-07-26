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
	"strings"
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

	mediaType, _, err := mime.ParseMediaType(fileHeaders.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed parsing content type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid content type", err)
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

	thumbNameBytes := make([]byte, 32)
	if _, err := rand.Read(thumbNameBytes); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed reading the uploaded file", err)
		return
	}

	thumbName := fmt.Sprintf("%v.%v", base64.RawURLEncoding.EncodeToString(thumbNameBytes), strings.Split(mediaType, "/")[1])
	thumbPath := filepath.Join(cfg.assetsRoot, thumbName)

	thumbData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed reading the uploaded file", err)
		return
	}

	if err := os.WriteFile(thumbPath, thumbData, os.ModePerm); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Parsing of media type failed", err)
		return
	}

	thumbURL := fmt.Sprintf("http://localhost:%v/%v", cfg.port, thumbPath)

	video.UpdatedAt = time.Now()
	video.ThumbnailURL = &thumbURL

	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Update video failed", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
