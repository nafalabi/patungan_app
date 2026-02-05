package handlers

// Breadcrumb represents a navigation trail
type Breadcrumb struct {
	Title string
	URL   string
}

// PageData represents the common data structure passed to templates
// Using this ensures type safety and consistency
type PageData struct {
	Title       string
	ActiveNav   string
	Breadcrumbs []Breadcrumb
	UserEmail   string
	UserUID     string
	Data        interface{} // Page-specific data
}
