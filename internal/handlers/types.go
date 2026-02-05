package handlers

// PageData represents the common data structure passed to templates
// Using this ensures type safety and consistency
type PageData struct {
	Title     string
	ActiveNav string
	UserEmail string
	UserUID   string
	Data      interface{} // Page-specific data
}
