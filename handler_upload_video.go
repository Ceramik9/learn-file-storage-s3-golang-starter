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
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Content is not a mp4 video file.", err)
		return
	}

	// create temp file
	tmpFile, err := os.CreateTemp("/tmp/", "video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't create temp file", err)
		return
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
		return
	}

	// create new file name
	fileName := createNewFileName("mp4")
	
	filePath := tmpFile.Name()
	filePathError := fmt.Sprintf("Couldn't get video aspect ratio from: %s", filePath)

	// get video aspect ratio
	aspectRatio, err := getVideoAspectRatio(filePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, filePathError, err)
		return
	}

	// create new video file processed for fast start
	newFilePath, err := processVideoForFastStart(filePath)
	processedFile, err := os.Open(newFilePath)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't open processed file", err)
	}
	defer os.Remove(newFilePath)
	defer processedFile.Close()


	fileNamePrefix := createVideoPrefix(aspectRatio)
	fileNameV2 := fmt.Sprintf("%s%s", fileNamePrefix, fileName)

	// add object to s3
	objectInput := s3.PutObjectInput {
		Bucket:      &cfg.s3Bucket, // bucket name
		Key:         &fileNameV2,   // file name
		Body:        processedFile, // file to upload
		ContentType: &mediaType,    // mime type (media type)
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &objectInput)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't add object to s3 bucket", err)
		return
	}

	// update video URL
	newVideoURL := fmt.Sprintf("%s%s",cfg.s3CfDistribution, fileNameV2)
	videoData.VideoURL = &newVideoURL
	
	// update db Videos
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


func createVideoPrefix(aspectRatio string) string {
	
	if aspectRatio == "16:9" {
		return "landscape/"
	} else {
		return "portrait/"
	}
}


