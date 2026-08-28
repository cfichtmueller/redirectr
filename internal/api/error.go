package api

import (
	"github.com/cfichtmueller/redirectr/internal/ec"
	"github.com/cfichtmueller/srv"
)

func responseFromError(err error) *srv.Response {
	e, ok := err.(*ec.Error)
	if ok {
		return srv.Respond().Status(e.StatusCode).Json(srv.ErrorDto{
			Code:    e.Code,
			Message: e.Message,
		})
	}
	return srv.Respond().Error(err)
}
