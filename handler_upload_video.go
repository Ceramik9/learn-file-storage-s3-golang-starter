package main

import (
	"fmt"
	"net/http"
	"mime"
	"os"
	"io"
	"encoding/base64"
	"crypto/rand"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/google/uuid"
	
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

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

	fmt.Println("uploading video", videoID, "by user", userID)
	
	// set max memory usage
	const maxMemory = 1 << 30 // 1 GB
	_ = http.MaxBytesReader(w, r.Body, maxMemory)

	// get video metadata
	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Video does not exist", err)
		return
	}
	
	// verify user owns the video
	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unathorized user", err)
		return
	}
	
	// form a readable file
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't form a file", err)
		return
	}
	defer file.Close()

	// validate media type
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Content is not a mp4 video file.", err)
		return
	}

	// create temp file
	tmpFile, err := os.CreateTemp("/tmp/", "video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't create temp file", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// copy video content to a temp file
	_, err = io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't copy file contents", err)
		return
	}
	
	// reset the tempFile's file pointer to the beginning
	_, err = tmpFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't reset file's pointer", err)
	}

	// create new file name
	fileName := createNewFileName("mp4")

	// add object to s3
	objectInput := s3.PutObjectInput {
		Bucket:      &cfg.s3Bucket, // bucket name
		Key:         &fileName,     // file name
		Body:        tmpFile,       // file to upload
		ContentType: &mediaType,    // mime type (media type)
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &objectInput)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't add object to s3 bucket", err)
	}

	// update video URL
	newVideoURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",cfg.s3Bucket, cfg.s3Region, fileName)
	videoData.VideoURL = &newVideoURL

	err = cfg.db.UpdateVideo(videoData)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Unable to save video metadata", err)
		return
		}

	respondWithJSON(w, http.StatusOK, videoData)
}

// helper function to generate new file name
func createNewFileName(fileExtention string) string {

	// create new byte slice
	newFileName := make([]byte, 32)
	// create new base64
	var RawURLEncoding = base64.URLEncoding.WithPadding(base64.NoPadding)
	// generate random file name
	rand.Read(newFileName)
	// encode new file name to base64
	encodedNewFileName := RawURLEncoding.EncodeToString(newFileName)
	// create new file name
	fileName := fmt.Sprintf("%s.%s", encodedNewFileName, fileExtention)

	return fileName
}


