package middlewares

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/containous/traefik/log"
	"github.com/containous/traefik/types"
	"github.com/vulcand/oxy/forward"
	"github.com/vulcand/oxy/utils"
)

//ErrorPagesHandler is a middleware that provides the custom error pages
type ErrorPagesHandler struct {
	HTTPCodeRanges     [][2]int
	BackendURL         string
	errorPageForwarder *forward.Forwarder
}

//NewErrorPagesHandler initializes the utils.ErrorHandler for the custom error pages
func NewErrorPagesHandler(errorPage types.ErrorPage, backendURL string) (*ErrorPagesHandler, error) {
	fwd, err := forward.New()
	if err != nil {
		return nil, err
	}

	//Break out the http status code ranges into a low int and high int
	//for ease of use at runtime
	var blocks [][2]int
	for _, block := range errorPage.Status {
		codes := strings.Split(block, "-")
		lowCode, err := strconv.Atoi(codes[0])
		if err != nil {
			return nil, err
		}
		highCode, err := strconv.Atoi(codes[1])
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, [2]int{lowCode, highCode})
	}
	return &ErrorPagesHandler{
			HTTPCodeRanges:     blocks,
			BackendURL:         backendURL + errorPage.Query,
			errorPageForwarder: fwd},
		nil
}

func (ep *ErrorPagesHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, next http.HandlerFunc) {
	recorder := NewRecorder()
	recorder.responseWriter = w
	next.ServeHTTP(recorder, req)

	//check the recorder code against the configured http status code ranges
	for _, block := range ep.HTTPCodeRanges {
		if recorder.Code >= block[0] && recorder.Code <= block[1] {
			log.Debugf("Caught HTTP Status Code %d, returning error page", recorder.Code)
			w.WriteHeader(recorder.Code)
			if newReq, err := http.NewRequest(http.MethodGet, ep.BackendURL, nil); err != nil {
				w.Write([]byte(http.StatusText(recorder.Code)))
			} else {
				ep.errorPageForwarder.ServeHTTP(w, newReq)
			}
			return
		}
	}

	//did not catch a configured status code so proceed with the request
	utils.CopyHeaders(w.Header(), recorder.Header())
	w.WriteHeader(recorder.Code)
	w.Write(recorder.Body.Bytes())
}
