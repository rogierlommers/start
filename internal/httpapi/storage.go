package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type uploadStorageFileResponse struct {
	Filename   string    `json:"filename"`
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

const maxUploadFilesPerRequest = 20

type storageFileResponse struct {
	Filename    string    `json:"filename"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

type uploadStorageFilesResponse struct {
	Status   string                    `json:"status"`
	Uploaded int                       `json:"uploaded"`
	Failed   int                       `json:"failed"`
	Results  []uploadStorageFileResult `json:"results"`
}

type uploadStorageFileResult struct {
	Filename   string    `json:"filename"`
	Path       string    `json:"path,omitempty"`
	Size       int64     `json:"size,omitempty"`
	UploadedAt time.Time `json:"uploaded_at,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// uploadStorageFile godoc
// @Summary Upload a file to server storage
// @Tags storage
// @Accept mpfd
// @Produce json
// @Param file formData file true "File to upload"
// @Success 201 {object} uploadStorageFileResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 409 {object} apiErrorResponse
// @Failure 413 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/storage/upload [post]
func (h handlers) uploadStorageFile(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "file is required"})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "failed to open uploaded file"})
		return
	}
	defer file.Close()

	stored, err := h.svc.UploadStorageFile(c.Request.Context(), service.UploadStorageFileInput{
		Filename:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		Reader:      file,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStorageInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid upload input"})
		case errors.Is(err, service.ErrStorageFileExists):
			c.JSON(http.StatusConflict, apiErrorResponse{Error: "file already exists"})
		case errors.Is(err, service.ErrStorageTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, apiErrorResponse{Error: "file too large"})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to store file"})
		}
		return
	}

	c.JSON(http.StatusCreated, uploadStorageFileResponse{
		Filename:   stored.Filename,
		Path:       stored.Path,
		Size:       stored.Size,
		UploadedAt: stored.UploadedAt,
	})
}

// uploadStorageFiles godoc
// @Summary Upload multiple files to server storage
// @Tags storage
// @Accept mpfd
// @Produce json
// @Param files formData file true "Files to upload (repeat field)"
// @Success 201 {object} uploadStorageFilesResponse
// @Success 207 {object} uploadStorageFilesResponse
// @Failure 400 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/storage/uploads [post]
func (h handlers) uploadStorageFiles(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid multipart form"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "at least one file is required in field 'files'"})
		return
	}
	if len(files) > maxUploadFilesPerRequest {
		c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "too many files in one request"})
		return
	}

	results := make([]uploadStorageFileResult, 0, len(files))
	uploaded := 0
	failed := 0

	for _, fileHeader := range files {
		item := uploadStorageFileResult{Filename: fileHeader.Filename}

		file, openErr := fileHeader.Open()
		if openErr != nil {
			item.Status = "failed"
			item.Error = "failed to open uploaded file"
			failed++
			results = append(results, item)
			continue
		}

		stored, uploadErr := h.svc.UploadStorageFile(c.Request.Context(), service.UploadStorageFileInput{
			Filename:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Reader:      file,
		})
		_ = file.Close()

		if uploadErr != nil {
			item.Status = "failed"
			switch {
			case errors.Is(uploadErr, service.ErrInvalidStorageInput):
				item.Error = "invalid upload input"
			case errors.Is(uploadErr, service.ErrStorageFileExists):
				item.Error = "file already exists"
			case errors.Is(uploadErr, service.ErrStorageTooLarge):
				item.Error = "file too large"
			default:
				item.Error = "failed to store file"
			}
			failed++
			results = append(results, item)
			continue
		}

		item.Status = "uploaded"
		item.Path = stored.Path
		item.Size = stored.Size
		item.UploadedAt = stored.UploadedAt
		uploaded++
		results = append(results, item)
	}

	statusCode := http.StatusCreated
	statusText := "uploaded"
	if failed > 0 {
		statusCode = http.StatusMultiStatus
		statusText = "partially_uploaded"
	}

	c.JSON(statusCode, uploadStorageFilesResponse{
		Status:   statusText,
		Uploaded: uploaded,
		Failed:   failed,
		Results:  results,
	})
}

// listStorageFiles godoc
// @Summary List uploaded storage files
// @Tags storage
// @Produce json
// @Success 200 {array} storageFileResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/storage/files [get]
func (h handlers) listStorageFiles(c *gin.Context) {
	files, err := h.svc.ListStorageFiles(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to list storage files"})
		return
	}

	resp := make([]storageFileResponse, len(files))
	for i, f := range files {
		resp[i] = storageFileResponse{
			Filename:    f.Filename,
			Path:        f.Path,
			Size:        f.Size,
			ContentType: f.ContentType,
			UploadedAt:  f.UploadedAt,
		}
	}

	c.JSON(http.StatusOK, resp)
}

// downloadStorageFile godoc
// @Summary Download a storage file
// @Tags storage
// @Produce application/octet-stream
// @Param filename path string true "Stored filename"
// @Success 200 {file} file
// @Failure 400 {object} apiErrorResponse
// @Failure 404 {object} apiErrorResponse
// @Failure 500 {object} apiErrorResponse
// @Router /api/storage/files/{filename} [get]
func (h handlers) downloadStorageFile(c *gin.Context) {
	opened, err := h.svc.OpenStorageFile(c.Request.Context(), c.Param("filename"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidStorageInput):
			c.JSON(http.StatusBadRequest, apiErrorResponse{Error: "invalid filename"})
		case errors.Is(err, service.ErrStorageNotFound):
			c.JSON(http.StatusNotFound, apiErrorResponse{Error: "file not found"})
		default:
			c.JSON(http.StatusInternalServerError, apiErrorResponse{Error: "failed to open file"})
		}
		return
	}
	defer opened.File.Close()

	if opened.ContentType != "" {
		c.Header("Content-Type", opened.ContentType)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}

	// Let browsers preview image files inline; keep download behavior for other content types.
	if strings.HasPrefix(opened.ContentType, "image/") {
		c.File(opened.Path)
		return
	}

	c.FileAttachment(opened.Path, opened.Filename)
}
