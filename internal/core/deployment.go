package core

type Deployment struct {
	EventID       string `bson:"_id" json:"eventId"`
	ProjectID     string `bson:"projectId" json:"-"`
	EnvironmentID string `bson:"environmentId" json:"-"`
	ApplicationID string `bson:"applicationId" json:"-"`
	DomainID      string `bson:"domainId" json:"-"`
	Phase         string `bson:"phase" json:"phase"`
	Host          string `bson:"host" json:"host"`
	Image         string `bson:"image" json:"-"`
	Error         string `bson:"error" json:"error,omitempty"`
}
