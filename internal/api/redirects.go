package api

import (
	"strconv"
	"time"

	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/uc"
	"github.com/cfichtmueller/srv"
)

func configureRedirectApi(s *srv.Server) {
	redirectsGroup := s.Group("/api/v1/redirects", authenticated)
	redirectsGroup.POST("", handleCreateRedirect)
	redirectsGroup.GET("", handleListRedirects)

	redirectGroup := redirectsGroup.Group("/{redirectId}", withRedirectFromPath)
	redirectGroup.GET("", handleGetRedirect)
	redirectGroup.PUT("", handleUpdateRedirect)
	redirectGroup.DELETE("", handleDeleteRedirect)
}

//
// Request/Response Types
//

type CreateRedirectRequest struct {
	SourceDomain string            `json:"sourceDomain"`
	TargetDomain string            `json:"targetDomain"`
	Status       string            `json:"status"`
	RedirectType string            `json:"redirectType"`
	UTMTags      *redirect.UTMTags `json:"utmTags,omitempty"`
}

func (r *CreateRedirectRequest) Validate() error {
	v := uc.RequireDomain("sourceDomain", r.SourceDomain, nil)
	v = uc.RequireDomain("targetDomain", r.TargetDomain, v)
	v = srv.RequireNotEmpty("status", r.Status, v)
	v = srv.RequireEnumValue("status", r.Status, redirect.Statuses, v)
	v = srv.RequireNotEmpty("redirectType", r.RedirectType, v)
	v = srv.RequireEnumValue("redirectType", r.RedirectType, redirect.RedirectTypes, v)
	return srv.Validate(v)
}

type UpdateRedirectRequest struct {
	SourceDomain string            `json:"sourceDomain"`
	TargetDomain string            `json:"targetDomain"`
	Status       string            `json:"status"`
	RedirectType string            `json:"redirectType"`
	UTMTags      *redirect.UTMTags `json:"utmTags,omitempty"`
}

func (r *UpdateRedirectRequest) Validate() error {
	v := uc.RequireDomain("sourceDomain", r.SourceDomain, nil)
	v = uc.RequireDomain("targetDomain", r.TargetDomain, v)
	v = srv.RequireNotEmpty("status", r.Status, v)
	v = srv.RequireEnumValue("status", r.Status, redirect.Statuses, v)
	v = srv.RequireNotEmpty("redirectType", r.RedirectType, v)
	v = srv.RequireEnumValue("redirectType", r.RedirectType, redirect.RedirectTypes, v)
	return srv.Validate(v)
}

type RedirectResponse struct {
	ID           string            `json:"id"`
	SourceDomain string            `json:"sourceDomain"`
	TargetDomain string            `json:"targetDomain"`
	Status       string            `json:"status"`
	RedirectType string            `json:"redirectType"`
	UTMTags      *redirect.UTMTags `json:"utmTags,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

type ListRedirectsResponse struct {
	Redirects []RedirectResponse `json:"redirects"`
	Total     int64              `json:"total"`
}

//
// Handlers
//

func handleCreateRedirect(c *srv.Context) *srv.Response {
	var req CreateRedirectRequest
	if r := c.BindJSON(&req); r != nil {
		return r
	}

	principal := contextMustGetPrincipal(c)
	redirect, err := uc.CreateRedirect(c, principal, uc.CreateRedirectCommand{
		SourceDomain: req.SourceDomain,
		TargetDomain: req.TargetDomain,
		Status:       req.Status,
		RedirectType: req.RedirectType,
		UTMTags:      req.UTMTags,
	})
	if err != nil {
		return responseFromError(err)
	}

	return srv.Respond().Json(RedirectResponse{
		ID:           redirect.ID,
		SourceDomain: redirect.SourceDomain,
		TargetDomain: redirect.TargetDomain,
		Status:       redirect.Status,
		RedirectType: redirect.RedirectType,
		UTMTags:      redirect.UTMTags,
		CreatedAt:    truncateTime(redirect.CreatedAt),
		UpdatedAt:    truncateTime(redirect.UpdatedAt),
	})
}

func handleListRedirects(c *srv.Context) *srv.Response {
	principal := contextMustGetPrincipal(c)

	// Parse query parameters
	filter := &redirect.Filter{}
	if q := c.Query("q"); q != "" {
		filter.Q = q
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && limit > 0 {
			filter.Limit = int(limit)
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil && offset >= 0 {
			filter.Offset = int(offset)
		}
	}

	redirects, err := uc.ListRedirects(c, principal, filter)
	if err != nil {
		return responseFromError(err)
	}

	total, err := uc.CountRedirects(c, principal, filter)
	if err != nil {
		return responseFromError(err)
	}

	response := ListRedirectsResponse{
		Redirects: make([]RedirectResponse, len(redirects)),
		Total:     total,
	}

	for i, r := range redirects {
		response.Redirects[i] = RedirectResponse{
			ID:           r.ID,
			SourceDomain: r.SourceDomain,
			TargetDomain: r.TargetDomain,
			Status:       r.Status,
			RedirectType: r.RedirectType,
			UTMTags:      r.UTMTags,
			CreatedAt:    truncateTime(r.CreatedAt),
			UpdatedAt:    truncateTime(r.UpdatedAt),
		}
	}

	return srv.Respond().Json(response)
}

func handleGetRedirect(c *srv.Context) *srv.Response {
	r := contextMustGetRedirect(c)

	return srv.Respond().Json(RedirectResponse{
		ID:           r.ID,
		SourceDomain: r.SourceDomain,
		TargetDomain: r.TargetDomain,
		Status:       r.Status,
		RedirectType: r.RedirectType,
		UTMTags:      r.UTMTags,
		CreatedAt:    truncateTime(r.CreatedAt),
		UpdatedAt:    truncateTime(r.UpdatedAt),
	}).
		LastModified(r.UpdatedAt).
		ETag(r.ETag)
}

func handleUpdateRedirect(c *srv.Context) *srv.Response {
	principal := contextMustGetPrincipal(c)
	r := contextMustGetRedirect(c)

	var req UpdateRedirectRequest
	if r := c.BindJSON(&req); r != nil {
		return r
	}

	redirect, err := uc.UpdateRedirect(c, principal, r, uc.UpdateRedirectCommand{
		SourceDomain: req.SourceDomain,
		TargetDomain: req.TargetDomain,
		Status:       req.Status,
		RedirectType: req.RedirectType,
		UTMTags:      req.UTMTags,
	})
	if err != nil {
		return responseFromError(err)
	}

	return srv.Respond().Json(RedirectResponse{
		ID:           redirect.ID,
		SourceDomain: redirect.SourceDomain,
		TargetDomain: redirect.TargetDomain,
		Status:       redirect.Status,
		RedirectType: redirect.RedirectType,
		UTMTags:      redirect.UTMTags,
		CreatedAt:    truncateTime(redirect.CreatedAt),
		UpdatedAt:    truncateTime(redirect.UpdatedAt),
	}).
		LastModified(redirect.UpdatedAt).
		ETag(redirect.ETag)
}

func handleDeleteRedirect(c *srv.Context) *srv.Response {
	principal := contextMustGetPrincipal(c)
	r := contextMustGetRedirect(c)
	if err := uc.DeleteRedirect(c, principal, r); err != nil {
		return responseFromError(err)
	}

	return srv.Respond().NoContent()
}
