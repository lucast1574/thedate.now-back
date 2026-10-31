package mongostore

import "go.mongodb.org/mongo-driver/v2/bson"

// Evaluated against the document before its editable fields are changed.
func publishedSnapshotExpression() bson.M {
	fields := bson.M{}
	for _, key := range []string{"backgroundImage", "opening", "celebration", "music", "templateId", "designMode", "title", "description", "template", "accentColor", "sections", "photoKeys", "audioKeys"} {
		fields[key] = "$" + key
	}
	return fields
}

func preservePublishedStage() bson.D {
	return bson.D{{Key: "$set", Value: bson.M{"publishedDesign": bson.M{"$cond": bson.A{
		bson.M{"$ne": bson.A{bson.M{"$ifNull": bson.A{"$publishedAt", nil}}, nil}},
		bson.M{"$ifNull": bson.A{"$publishedDesign", publishedSnapshotExpression()}},
		"$$REMOVE",
	}}}}}
}

func initialPublishedVersion() bson.M {
	return bson.M{"$cond": bson.A{
		bson.M{"$eq": bson.A{bson.M{"$ifNull": bson.A{"$publishedDesign", nil}}, nil}},
		bson.M{"$ifNull": bson.A{"$designVersion", 0}},
		bson.M{"$ifNull": bson.A{"$publishedDesignVersion", 0}},
	}}
}
