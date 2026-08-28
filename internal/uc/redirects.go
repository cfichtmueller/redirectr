package uc

import (
	"context"

	"github.com/cfichtmueller/redirectr/internal/domain/redirect"
	"github.com/cfichtmueller/redirectr/internal/infra/audit"
	"github.com/cfichtmueller/redirectr/internal/infra/auth"
)

//
// Commands
//

type CreateRedirectCommand struct {
	SourceDomain string            `json:"sourceDomain"`
	TargetDomain string            `json:"targetDomain"`
	Status       string            `json:"status"`
	RedirectType string            `json:"redirectType"`
	UTMTags      *redirect.UTMTags `json:"utmTags,omitempty"`
}

type UpdateRedirectCommand struct {
	SourceDomain string            `json:"sourceDomain"`
	TargetDomain string            `json:"targetDomain"`
	Status       string            `json:"status"`
	RedirectType string            `json:"redirectType"`
	UTMTags      *redirect.UTMTags `json:"utmTags,omitempty"`
}

//
// Use Cases
//

func CreateRedirect(c context.Context, principal *auth.Principal, cmd CreateRedirectCommand) (*redirect.Redirect, error) {
	r, err := redirect.Create(c, principal, redirect.CreateCommand{
		SourceDomain: cmd.SourceDomain,
		TargetDomain: cmd.TargetDomain,
		Status:       cmd.Status,
		RedirectType: cmd.RedirectType,
		UTMTags:      cmd.UTMTags,
	})
	if err != nil {
		return nil, err
	}

	if err := audit.WriteEntityData(c, principal, AuditEventRedirectCreated, r.ID, nil, map[string]any{
		"sourceDomain": r.SourceDomain,
		"targetDomain": r.TargetDomain,
		"status":       r.Status,
		"redirectType": r.RedirectType,
		"utmTags":      r.UTMTags,
	}); err != nil {
		return nil, err
	}

	return r, nil
}

func ListRedirects(c context.Context, principal *auth.Principal, filter *redirect.Filter) ([]*redirect.Redirect, error) {
	if filter == nil {
		filter = &redirect.Filter{}
	}
	filter.UserID = principal.ID
	return redirect.FindMany(c, filter)
}

func UpdateRedirect(c context.Context, principal *auth.Principal, r *redirect.Redirect, cmd UpdateRedirectCommand) (*redirect.Redirect, error) {
	updated, err := redirect.Update(c, principal, r, redirect.UpdateCommand{
		SourceDomain: cmd.SourceDomain,
		TargetDomain: cmd.TargetDomain,
		Status:       cmd.Status,
		RedirectType: cmd.RedirectType,
		UTMTags:      cmd.UTMTags,
	})
	if err != nil {
		return nil, err
	}

	if err := audit.WriteEntityData(c, principal, AuditEventRedirectUpdated, updated.ID, nil, map[string]any{
		"sourceDomain": updated.SourceDomain,
		"targetDomain": updated.TargetDomain,
		"status":       updated.Status,
		"redirectType": updated.RedirectType,
		"utmTags":      updated.UTMTags,
	}); err != nil {
		return nil, err
	}

	return updated, nil
}

func DeleteRedirect(c context.Context, principal *auth.Principal, r *redirect.Redirect) error {
	if err := redirect.Delete(c, principal, r.ID); err != nil {
		return err
	}

	if err := audit.WriteEntity(c, principal, AuditEventRedirectDeleted, r.ID, nil); err != nil {
		return err
	}

	return nil
}

func CountRedirects(c context.Context, principal *auth.Principal, filter *redirect.Filter) (int64, error) {
	if filter == nil {
		filter = &redirect.Filter{}
	}
	filter.UserID = principal.ID
	return redirect.Count(c, filter)
}
