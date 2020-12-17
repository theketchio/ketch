package chart

type platform struct {
	Name        string `json:"name"`
	Image       string `json:"image,omitempty"`
	Description string `json:"description,omitempty"`
}
