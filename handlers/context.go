package handlers

import (
	"skills-hub/db"
	"skills-hub/models"
)

type RequestContext struct {
	Session       *sessionData
	User          *models.User
	CurrentTenant *models.UserTenant
	TenantOptions []models.UserTenant
}

func buildRequestContext(userID int64, sess *sessionData) (*RequestContext, error) {
	ctx := &RequestContext{Session: sess}
	if sess == nil || userID == 0 {
		return ctx, nil
	}

	user, err := db.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	ctx.User = user

	tenantOptions, err := db.ListUserTenants(userID)
	if err != nil {
		return nil, err
	}
	ctx.TenantOptions = tenantOptions

	if sess.CurrentTenantID > 0 {
		currentTenant, err := db.GetUserTenant(userID, sess.CurrentTenantID)
		if err != nil {
			return nil, err
		}
		ctx.CurrentTenant = currentTenant
	}

	return ctx, nil
}
