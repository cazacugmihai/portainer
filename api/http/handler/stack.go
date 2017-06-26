package handler

import (
	"encoding/json"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/portainer/portainer"
	httperror "github.com/portainer/portainer/http/error"
	"github.com/portainer/portainer/http/security"

	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

// StackHandler represents an HTTP API handler for managing Stack.
type StackHandler struct {
	*mux.Router
	Logger          *log.Logger
	FileService     portainer.FileService
	StackService    portainer.StackService
	EndpointService portainer.EndpointService
	StackManager    portainer.StackManager
}

// NewStackHandler returns a new instance of StackHandler.
func NewStackHandler(bouncer *security.RequestBouncer) *StackHandler {
	h := &StackHandler{
		Router: mux.NewRouter(),
		Logger: log.New(os.Stderr, "", log.LstdFlags),
	}
	h.Handle("/{endpointId}/stacks",
		bouncer.AuthenticatedAccess(http.HandlerFunc(h.handlePostStack))).Methods(http.MethodPost)
	h.Handle("/{endpointId}/stacks/{id}/up",
		bouncer.AuthenticatedAccess(http.HandlerFunc(h.handleStackOperationUp))).Methods(http.MethodPost)

	return h
}

// handlePostStack handles POST requests on /:endpointID/stacks
func (handler *StackHandler) handlePostStack(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	var req postStacksRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidJSON, http.StatusBadRequest, handler.Logger)
		return
	}

	_, err = govalidator.ValidateStruct(req)
	if err != nil {
		httperror.WriteErrorResponse(w, ErrInvalidRequestFormat, http.StatusBadRequest, handler.Logger)
		return
	}

	stack := &portainer.Stack{
		Name:       req.Name,
		EndpointID: portainer.EndpointID(endpointID),
	}

	projectPath, err := handler.FileService.StoreComposeFile(stack.Name, req.ComposeFileContent)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	if req.EnvFileContent != "" {
		err = handler.FileService.StoreComposeEnvFile(stack.Name, req.EnvFileContent)
		if err != nil {
			httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
			return
		}
	}

	stack.ProjectPath = projectPath
	err = handler.StackService.CreateStack(stack)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	encodeJSON(w, &postStacksResponse{ID: int(stack.ID)}, handler.Logger)
}

type postStacksRequest struct {
	Name               string `valid:"required"`
	ComposeFileContent string `valid:"required"`
	EnvFileContent     string `valid:""`
}

type postStacksResponse struct {
	ID int `json:"Id"`
}

func (handler *StackHandler) handleStackOperationUp(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	endpointID, err := strconv.Atoi(vars["endpointId"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	endpoint, err := handler.EndpointService.Endpoint(portainer.EndpointID(endpointID))
	if err == portainer.ErrEndpointNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	stackID, err := strconv.Atoi(vars["id"])
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusBadRequest, handler.Logger)
		return
	}

	stack, err := handler.StackService.Stack(portainer.StackID(stackID))
	if err == portainer.ErrStackNotFound {
		httperror.WriteErrorResponse(w, err, http.StatusNotFound, handler.Logger)
		return
	} else if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}

	err = handler.StackManager.Start(stack, endpoint)
	if err != nil {
		httperror.WriteErrorResponse(w, err, http.StatusInternalServerError, handler.Logger)
		return
	}
}
