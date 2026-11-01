package core

import (
	"errors"
	"math"
	"strings"
)

type VenueArea struct {
	Rotation float64 `bson:"rotation" json:"rotation"`
	ID       string  `bson:"id" json:"id"`
	Name     string  `bson:"name" json:"name"`
	Kind     string  `bson:"kind" json:"kind"`
	X        float64 `bson:"x" json:"x"`
	Y        float64 `bson:"y" json:"y"`
	Width    float64 `bson:"width" json:"width"`
	Height   float64 `bson:"height" json:"height"`
}

func ValidateVenueAreas(plan SeatingPlan) error {
	if len(plan.Areas) > 30 {
		return errors.New("too many venue areas")
	}
	seen := map[string]bool{}
	for _, table := range plan.Tables {
		seen[table.ID] = true
	}
	for _, area := range plan.Areas {
		validKind := area.Kind == "snacks" || area.Kind == "bar" || area.Kind == "stage" || area.Kind == "dance" || area.Kind == "lounge" || area.Kind == "photo" || area.Kind == "entrance"
		if area.ID == "" || len(area.ID) > 64 || seen[area.ID] || strings.TrimSpace(area.Name) == "" || len(area.Name) > 100 || !validKind || !bounded(area.Rotation, -180, 180) || !bounded(area.Width, 80, 800) || !bounded(area.Height, 60, 600) || !venueAreaInsideRoom(area, plan) {
			return errors.New("invalid or duplicate venue area")
		}
		seen[area.ID] = true
	}
	return nil
}

func venueAreaInsideRoom(area VenueArea, plan SeatingPlan) bool {
	// Keep previously saved layouts valid under their original, unrotated bounds.
	if bounded(area.X, 0, plan.Width-area.Width) && bounded(area.Y, 0, plan.Height-area.Height) {
		return true
	}
	radians := area.Rotation * math.Pi / 180
	cos, sin := math.Abs(math.Cos(radians)), math.Abs(math.Sin(radians))
	offsetX := (area.Width*cos + area.Height*sin - area.Width) / 2
	offsetY := (area.Width*sin + area.Height*cos - area.Height) / 2
	const epsilon = 1e-7
	return bounded(area.X, offsetX-epsilon, plan.Width-area.Width-offsetX+epsilon) &&
		bounded(area.Y, offsetY-epsilon, plan.Height-area.Height-offsetY+epsilon)
}
