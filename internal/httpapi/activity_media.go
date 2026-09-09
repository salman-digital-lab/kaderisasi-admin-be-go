package httpapi

import (
	"kaderisasi/admin/internal/activity"
	"net/http"
)

func (s *Server) registerActivityMedia() {
	service := activity.Service{Pool: s.Pool, Storage: s.Storage}
	s.register("activities_controller", "deleteImage", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.DeleteImageRequest](w, r, "deleteActivityImageValidator")
		if !ok {
			return nil
		}
		result, err := service.DeleteImage(r.Context(), pathID(r, "id"), data)
		if err != nil {
			return err
		}
		reply(w, 200, "DELETE_IMAGE_SUCCESS", result)
		return nil
	})
	s.register("activities_controller", "reorderImages", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.ImageOrderRequest](w, r, "reorderActivityImagesValidator")
		if !ok {
			return nil
		}
		result, err := service.ReorderImages(r.Context(), pathID(r, "id"), data)
		if err != nil {
			return err
		}
		reply(w, 200, "REORDER_IMAGES_SUCCESS", result)
		return nil
	})
}
