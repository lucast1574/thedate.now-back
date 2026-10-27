package core

type ImageBackground struct {
	PhotoKey string  `bson:"photoKey" json:"photoKey"`
	Fit      string  `bson:"fit" json:"fit"`
	Position string  `bson:"position" json:"position"`
	Opacity  float64 `bson:"opacity" json:"opacity"`
}

func ValidBackground(bg *ImageBackground, photos []string) bool {
	return bg == nil || (validPhoto(bg.PhotoKey, photos) && (bg.Fit == "cover" || bg.Fit == "contain") && (bg.Position == "center" || bg.Position == "top" || bg.Position == "bottom") && bounded(bg.Opacity, 0, 1))
}
