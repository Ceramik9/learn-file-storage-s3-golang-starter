package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
	"strings"
	"path/filepath"
	"crypto/rand"
	"encoding/base64"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {

	// get video id from request url
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// get user token
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	// validate user token
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}
	
	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)
	
	// set max memory usage
	const maxMemory = 10 << 20

	// parse miltipart request
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart format", err)
		return
	}
	
	// form a readable file
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	videoMediaType := header.Header.Get("Content-Type")

	if videoMediaType != "image/jpeg" && videoMediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Content is not an image.", err)
		return
	}
	
	// get file extention from Media-Type request header
	fileExtention := strings.Split(videoMediaType, "/")
	
	// create new byte slice
	newFileName := make([]byte, 32)

	// create new base64
	var RawURLEncoding = base64.URLEncoding.WithPadding(base64.NoPadding)
	
	// generate random file name
	rand.Read(newFileName)

	// encode new file name to base64
	encodedNewFileName := RawURLEncoding.EncodeToString(newFileName)

	// create new file name
	fileName := fmt.Sprintf("%s.%s", encodedNewFileName, fileExtention[1])
	
	filePath := filepath.Join(cfg.assetsRoot, fileName)
	
	newFile, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to create a file", err)
		return
	}
	
	_, err = io.Copy(newFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to write to a file", err)
		return
	}

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video does not exist", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unathorized user", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, fileName)
	videoData.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to save video metadata", err)
	return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}









