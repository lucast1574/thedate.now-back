package mail

func AccessInvitation(kind, inviter, title, url string) string {
	content, err := Render(kind, "invitation", TemplateData{Inviter: inviter, Title: title, ActionURL: url})
	if err != nil {
		return ""
	}
	return content.HTML
}
