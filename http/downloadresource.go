package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/patdowney/downloaderd/api"
	"github.com/patdowney/downloaderd/common"
	"github.com/patdowney/downloaderd/download"
	"io"
	"log"
	"net/http"
)

type DownloadResource struct {
	Clock           common.Clock
	DownloadService *download.DownloadService
	router          *mux.Router
}

func NewDownloadResource(downloadService *download.DownloadService) *DownloadResource {
	return &DownloadResource{
		Clock:           &common.RealClock{},
		DownloadService: downloadService}
}

func (r *DownloadResource) RegisterRoutes(parentRouter *mux.Router) {
	parentRouter.HandleFunc("/", r.Index()).Methods("GET", "HEAD")
	// regexp matches ids that look like '8671301b-49fa-416c-4bc0-2869963779e5'
	parentRouter.HandleFunc("/{id:[a-f0-9-]{36}}", r.Get()).Methods("GET", "HEAD").Name("download")
	parentRouter.HandleFunc("/{id:[a-f0-9-]{36}}/data", r.GetData()).Methods("GET", "HEAD").Name("download-data")

	r.router = parentRouter
}

func (r *DownloadResource) WrapError(err error) *api.Error {
	return api.NewError(common.NewErrorWrapper(err, r.Clock.Now()))
}

func (r *DownloadResource) Index() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		downloadList, err := r.DownloadService.ListAll()

		encoder := json.NewEncoder(rw)
		rw.Header().Set("Content-Type", "application/json")

		if err != nil {
			log.Printf("server-error: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			encErr := encoder.Encode(r.WrapError(err))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		} else {
			rw.WriteHeader(http.StatusOK)

			encErr := encoder.Encode(api.NewDownloadList(&downloadList))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		}
	}
}

func (r *DownloadResource) GetData() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		downloadId := vars["id"]

		download, err := r.DownloadService.FindById(downloadId)
		encoder := json.NewEncoder(rw)

		if err != nil {
			log.Printf("server-error: %v", err)
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusInternalServerError)
			encErr := encoder.Encode(r.WrapError(err))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		} else if download != nil {
			if download.Finished {
				bufferedReader, err := r.DownloadService.GetReader(download)
				if err != nil {
					log.Printf("server-error: %v", err)
					rw.Header().Set("Content-Type", "application/json")
					rw.WriteHeader(http.StatusInternalServerError)
					encErr := encoder.Encode(r.WrapError(err))
					if encErr != nil {
						log.Printf("encoder-error: %v", encErr)
					}
				}

				meta := download.Metadata

				if meta.MimeType != "" {
					rw.Header().Set("Content-Type", meta.MimeType)
				}
				if meta.Size != 0 {
					rw.Header().Set("Content-Length", fmt.Sprintf("%d", meta.Size))
				} else {
					rw.Header().Set("Content-Length", fmt.Sprintf("%d", download.Status.BytesRead))
				}
				rw.WriteHeader(http.StatusOK)

				io.Copy(rw, bufferedReader)
			} else {
				rw.WriteHeader(http.StatusNoContent)
			}
		} else {
			rw.WriteHeader(http.StatusNotFound)
		}

	}
}

func (r *DownloadResource) Get() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		downloadId := vars["id"]

		download, err := r.DownloadService.FindById(downloadId)

		encoder := json.NewEncoder(rw)
		rw.Header().Set("Content-Type", "application/json")

		if err != nil {
			log.Printf("server-error: %v", err)
			rw.WriteHeader(http.StatusInternalServerError)
			encErr := encoder.Encode(r.WrapError(err))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		} else if download != nil {
			rw.WriteHeader(http.StatusOK)
			encErr := encoder.Encode(api.NewDownload(download))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		} else {
			errMessage := fmt.Sprintf("Unable to find order with id:%s", downloadId)
			log.Printf("server-error: %v", errMessage)

			rw.WriteHeader(http.StatusNotFound)
			encErr := encoder.Encode(errors.New(errMessage))
			if encErr != nil {
				log.Printf("encoder-error: %v", encErr)
			}
		}
	}
}
