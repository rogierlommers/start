package httpapi

import (
	"errors"
	"net/http"
	"time"

	"start/internal/service"

	"github.com/gin-gonic/gin"
)

type incidentManagerDutyResponse struct {
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	AllDay      bool      `json:"all_day"`
}

type incidentManagerOverviewResponse struct {
	Timezone   string                        `json:"timezone"`
	RangeStart time.Time                     `json:"range_start"`
	RangeEnd   time.Time                     `json:"range_end"`
	Duties     []incidentManagerDutyResponse `json:"duties"`
}

// getIncidentManagerOverview godoc
// @Summary Get upcoming incident-manager duties
// @Tags incident-manager
// @Produce json
// @Security ApiBasicAuth
// @Success 200 {object} incidentManagerOverviewResponse
// @Failure 502 {object} apiErrorResponse
// @Failure 503 {object} apiErrorResponse
// @Router /api/incident-manager [get]
func (h handlers) getIncidentManagerOverview(c *gin.Context) {
	overview, err := h.svc.GetIncidentManagerOverview(c.Request.Context(), time.Now())
	if err != nil {
		if errors.Is(err, service.ErrIncidentManagerDisabled) {
			c.JSON(http.StatusServiceUnavailable, apiErrorResponse{Error: "incident manager calendar is not configured"})
			return
		}
		c.JSON(http.StatusBadGateway, apiErrorResponse{Error: "failed to load incident manager calendar"})
		return
	}

	duties := make([]incidentManagerDutyResponse, len(overview.Duties))
	for i, duty := range overview.Duties {
		duties[i] = incidentManagerDutyResponse{
			Summary:     duty.Summary,
			Description: duty.Description,
			Location:    duty.Location,
			Start:       duty.Start,
			End:         duty.End,
			AllDay:      duty.AllDay,
		}
	}

	c.JSON(http.StatusOK, incidentManagerOverviewResponse{
		Timezone:   overview.Timezone,
		RangeStart: overview.RangeStart,
		RangeEnd:   overview.RangeEnd,
		Duties:     duties,
	})
}
