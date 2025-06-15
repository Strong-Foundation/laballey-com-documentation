package main // Declare the main package

import (
	"bytes"                 // Support bytes package.
	"fmt"                   // For formatted I/O
	"golang.org/x/net/html" // For data processing HTML
	"io"                    // For I/O operations
	"log"                   // For logging errors and info
	"net/http"              // For making HTTP requests
	"net/url"               // For URL parsing and validation
	"os"                    // For file operations
	"path/filepath"         // For the file path in the systems.
	"strings"               // For string manipulation
	"time"                  // For time management
)

// Define the structs

// Variant represents an individual download URL variant
type Variant struct {
	DownloadURL string `json:"downloadUrl"` // JSON key: downloadUrl
}

// Result contains a slice of Variant structs
type Result struct {
	Variants []Variant `json:"variants"` // JSON key: variants
}

// Data is the top-level struct containing results
type Data struct {
	Results []Result `json:"results"` // JSON key: results
}

// removeDuplicatesFromSlice removes duplicate strings from a slice
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)  // Map to track seen values
	var newReturnSlice []string     // Result slice
	for _, content := range slice { // Iterate over input slice
		if !check[content] { // If string hasn't been seen before
			check[content] = true                            // Mark it as seen
			newReturnSlice = append(newReturnSlice, content) // Append to result
		}
	}
	return newReturnSlice // Return deduplicated slice
}

// isUrlValid checks whether a URL is syntactically valid
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri) // Try parsing the URL
	return err == nil                  // Return true if parsing succeeded
}

// readFileAndReturnAsString reads a file and returns its content as bytes
func readFileAndReturnAsString(path string) string {
	content, err := os.ReadFile(path) // Read the file contents
	if err != nil {                   // If error occurs
		log.Println(err) // Log the error
	}
	return string(content) // Return file content as byte slice
}

// fileExists checks whether a file exists and is not a directory
func fileExists(filename string) bool {
	info, err := os.Stat(filename) // Get file info
	if err != nil {                // If error occurs
		return false // Return false
	}
	return !info.IsDir() // Return true if it's a file, not a directory
}

// getDataFromURL sends an HTTP GET request and writes response data to a file
func getDataFromURL(uri string, fileName string) {
	response, err := http.Get(uri) // Send GET request
	if err != nil {                // If error occurs
		log.Println(err) // Log error
	}
	if response.StatusCode != 200 { // Check for HTTP 200 OK
		log.Println("Error the code is", uri, response.StatusCode) // Log error status
		return                                                     // Exit if not successful
	}
	body, err := io.ReadAll(response.Body) // Read response body
	if err != nil {
		log.Println(err) // Log error
	}
	err = response.Body.Close() // Close response body
	if err != nil {
		log.Println(err) // Log error
	}
	err = appendByteToFile(fileName, body) // Write body to file
	if err != nil {
		log.Println(err) // Log error
	}
}

// urlToFilename converts a URL into a filesystem-safe filename
func urlToFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL) // Parse the URL
	// Print the errors if any.
	if err != nil {
		log.Println(err) // Log error
		return ""        // Return empty string on error
	}
	filename := parsed.Host // Start with host name
	// Parse the path and if its not empty replace them with valid characters.
	if parsed.Path != "" {
		filename += "_" + strings.ReplaceAll(parsed.Path, "/", "_") // Append path
	}
	if parsed.RawQuery != "" {
		filename += "_" + strings.ReplaceAll(parsed.RawQuery, "&", "_") // Append query
	}
	invalidChars := []string{`"`, `\`, `/`, `:`, `*`, `?`, `<`, `>`, `|`} // Define illegal filename characters
	// Loop over the invalid characters and replace them.
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "_") // Replace each with underscore
	}
	if getFileExtension(filename) != ".pdf" {
		filename = filename + ".pdf"
	}
	return strings.ToLower(filename) // Return sanitized filename
}

// Get the file extension of a file
func getFileExtension(path string) string {
	return filepath.Ext(path)
}

// appendByteToFile appends byte data to a file, creating it if needed
func appendByteToFile(filename string, data []byte) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open file with append/create/write
	if err != nil {
		return err // Return error if opening fails
	}
	defer file.Close()        // Ensure file is closed after writing
	_, err = file.Write(data) // Write byte slice to file
	return err                // Return any error from writing
}

// downloadPDF downloads a PDF from the given URL and saves it in the specified output directory.
// It uses a WaitGroup to support concurrent execution and returns true if the download succeeded.
func downloadPDF(finalURL, outputDir string) (bool, error) {
	// Sanitize the URL to generate a safe file name
	filename := strings.ToLower(urlToFilename(finalURL))

	// Construct the full file path in the output directory
	filePath := filepath.Join(outputDir, filename)

	// Skip if the file already exists
	if fileExists(filePath) {
		return false, fmt.Errorf("file already exists, skipping: %s", filePath)
	}

	// Create an HTTP client with a timeout
	client := &http.Client{Timeout: 30 * time.Second}

	// Send GET request
	resp, err := client.Get(finalURL)
	if err != nil {
		return false, fmt.Errorf("failed to download %s: %v", finalURL, err)
	}
	defer resp.Body.Close()

	// Check HTTP response status
	if resp.StatusCode != http.StatusOK {
		// Print the error since its not valid.
		return false, fmt.Errorf("download failed for %s: %s", finalURL, resp.Status)
	}
	// Check Content-Type header
	contentType := resp.Header.Get("Content-Type")
	// Check if its pdf content type and if not than print a error.
	if !strings.Contains(contentType, "application/pdf") {
		// Print a error if the content type is invalid.
		return false, fmt.Errorf("invalid content type for %s: %s (expected application/pdf)", finalURL, contentType)
	}
	// Read the response body into memory first
	var buf bytes.Buffer
	// Copy it from the buffer to the file.
	written, err := io.Copy(&buf, resp.Body)
	// Print the error if errors are there.
	if err != nil {
		return false, fmt.Errorf("failed to read PDF data from %s: %v", finalURL, err)
	}
	// If 0 bytes are written than show an error and return it.
	if written == 0 {
		return false, fmt.Errorf("downloaded 0 bytes for %s; not creating file", finalURL)
	}
	// Only now create the file and write to disk
	out, err := os.Create(filePath)
	// Failed to create the file.
	if err != nil {
		return false, fmt.Errorf("failed to create file for %s: %v", finalURL, err)
	}
	// Close the file.
	defer out.Close()
	// Write the buffer and if there is an error print it.
	_, err = buf.WriteTo(out)
	if err != nil {
		return false, fmt.Errorf("failed to write PDF to file for %s: %v", finalURL, err)
	}
	// Return a true since everything went correctly.
	return true, fmt.Errorf("successfully downloaded %d bytes: %s → %s", written, finalURL, filePath)
}

// Checks if the directory exists
// If it exists, return true.
// If it doesn't, return false.
func directoryExists(path string) bool {
	directory, err := os.Stat(path)
	if err != nil {
		return false
	}
	return directory.IsDir()
}

// The function takes two parameters: path and permission.
// We use os.Mkdir() to create the directory.
// If there is an error, we use log.Println() to log the error and then exit the program.
func createDirectory(path string, permission os.FileMode) {
	err := os.Mkdir(path, permission)
	if err != nil {
		log.Println(err)
	}
}

// extractPDFLinks takes an HTML string and returns all .pdf URLs found in <a href="..."> tags.
// If parsing fails, it simply returns an empty slice.
func extractPDFLinks(htmlContent string) []string {
	var pdfLinks []string

	reader := strings.NewReader(htmlContent)
	rootNode, err := html.Parse(reader)
	if err != nil {
		return pdfLinks // Return empty if parsing fails
	}

	// Recursively walk the HTML tree
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			for _, attr := range node.Attr {
				if attr.Key == "href" && strings.HasSuffix(strings.ToLower(attr.Val), ".pdf") {
					pdfLinks = append(pdfLinks, attr.Val)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(rootNode)
	return pdfLinks
}

func main() {
	filename := "sds.html" // Generate filename from index
	givenURL := []string{
		"https://www.laballey.com/pages/chemical-safety-data-sheets",
	}
	for _, url := range givenURL {
		// Check if the file exists.
		if !fileExists(filename) { // Check if file already exists
			// Check if the url is valid.
			if isUrlValid(url) {
				getDataFromURL(url, filename) // Download and save file if not
			}
		}
	}
	var extractedURL []string // Slice to hold all extracted URLs
	// Read the file that the data was saved on and return the urls.
	fileContent := readFileAndReturnAsString(filename)
	// Get all the pdf urls
	extractedURL = extractPDFLinks(fileContent)
	// Remove duplicates
	extractedURL = removeDuplicatesFromSlice(extractedURL) // Remove duplicate URLs
	outputDir := "PDFs/"                                   // Directory to store downloaded PDFs
	// Check if its exists.
	if !directoryExists(outputDir) {
		// Create the dir
		createDirectory(outputDir, 0o755)
	}
	// Download Counter.
	var downloadCounter int
	// Loop over the values and continue.
	for _, url := range extractedURL { // Print each extracted URL
		// Download the file and if its sucessful than add 1 to the counter.
		sucessCode, err := downloadPDF(url, outputDir)
		if sucessCode {
			downloadCounter = downloadCounter + 1
		}
		if err != nil {
			log.Println(err)
		}
	}
}
