package mock

import (
	"google.golang.org/api/drive/v3"
)

// MockStorage is a test implementation of the Storage interface that allows
// complete control over method return values for unit testing.
// Each method can be configured to return specific values or errors as needed.
type MockStorage struct {
	// GenerateDownloadURL mock configuration
	GenerateDownloadURLFunc func(driveID string) string

	// ExtractFileIDFromURL mock configuration
	ExtractFileIDFromURLFunc func(url string) string

	// GetFiles mock configuration
	GetFilesFunc  func(query string, mostRecent bool) ([]*drive.File, error)
	GetFilesError error
	GetFilesFiles []*drive.File

	// GetMostRecentFile mock configuration
	GetMostRecentFileFunc func(files []*drive.File) *drive.File
	GetMostRecentFileFile *drive.File

	// FileExists mock configuration
	FileExistsFunc   func(fileID string) (bool, error)
	FileExistsResult bool
	FileExistsError  error

	// DeleteFile mock configuration
	DeleteFileFunc  func(fileID string) error
	DeleteFileError error

	// DownloadFile mock configuration
	DownloadFileFunc    func(fileID string) (string, error)
	DownloadFileContent string
	DownloadFileError   error

	// DownloadFileToTemp mock configuration
	DownloadFileToTempFunc  func(fileID string) (string, error)
	DownloadFileToTempPath  string
	DownloadFileToTempError error

	// UploadFile mock configuration
	UploadFileFunc  func(filePath, filename, mimeType string) (string, error)
	UploadFileID    string
	UploadFileError error

	// UploadString mock configuration
	UploadStringFunc  func(content, filename, mimeType, fileID string) (string, error)
	UploadStringID    string
	UploadStringError error

	// Call tracking for verification
	GenerateDownloadURLCalls  []string
	ExtractFileIDFromURLCalls []string
	GetFilesCalls             []GetFilesCall
	GetMostRecentFileCalls    [][]*drive.File
	FileExistsCalls           []string
	DeleteFileCalls           []string
	DownloadFileCalls         []string
	DownloadFileToTempCalls   []string
	UploadFileCalls           []UploadFileCall
	UploadStringCalls         []UploadStringCall
}

// Call tracking structs
type GetFilesCall struct {
	Query      string
	MostRecent bool
}

type UploadFileCall struct {
	FilePath string
	Filename string
	MimeType string
}

type UploadStringCall struct {
	Content  string
	Filename string
	MimeType string
	FileID   string
}

// NewMockStorage creates a new MockStorage with reasonable defaults.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		GenerateDownloadURLCalls:  make([]string, 0),
		ExtractFileIDFromURLCalls: make([]string, 0),
		GetFilesCalls:             make([]GetFilesCall, 0),
		GetMostRecentFileCalls:    make([][]*drive.File, 0),
		FileExistsCalls:           make([]string, 0),
		DeleteFileCalls:           make([]string, 0),
		DownloadFileCalls:         make([]string, 0),
		DownloadFileToTempCalls:   make([]string, 0),
		UploadFileCalls:           make([]UploadFileCall, 0),
		UploadStringCalls:         make([]UploadStringCall, 0),
	}
}

// GenerateDownloadURL implements Storage interface
func (m *MockStorage) GenerateDownloadURL(driveID string) string {
	m.GenerateDownloadURLCalls = append(m.GenerateDownloadURLCalls, driveID)
	if m.GenerateDownloadURLFunc != nil {
		return m.GenerateDownloadURLFunc(driveID)
	}
	return "https://mock-download-url.com/" + driveID
}

// ExtractFileIDFromURL implements Storage interface
func (m *MockStorage) ExtractFileIDFromURL(url string) string {
	m.ExtractFileIDFromURLCalls = append(m.ExtractFileIDFromURLCalls, url)
	if m.ExtractFileIDFromURLFunc != nil {
		return m.ExtractFileIDFromURLFunc(url)
	}
	return "mock-file-id"
}

// GetFiles implements Storage interface
func (m *MockStorage) GetFiles(query string, mostRecent bool) ([]*drive.File, error) {
	m.GetFilesCalls = append(m.GetFilesCalls, GetFilesCall{
		Query:      query,
		MostRecent: mostRecent,
	})
	if m.GetFilesFunc != nil {
		return m.GetFilesFunc(query, mostRecent)
	}
	if m.GetFilesError != nil {
		return nil, m.GetFilesError
	}
	return m.GetFilesFiles, nil
}

// GetMostRecentFile implements Storage interface
func (m *MockStorage) GetMostRecentFile(files []*drive.File) *drive.File {
	m.GetMostRecentFileCalls = append(m.GetMostRecentFileCalls, files)
	if m.GetMostRecentFileFunc != nil {
		return m.GetMostRecentFileFunc(files)
	}
	return m.GetMostRecentFileFile
}

// FileExists implements Storage interface
func (m *MockStorage) FileExists(fileID string) (bool, error) {
	m.FileExistsCalls = append(m.FileExistsCalls, fileID)
	if m.FileExistsFunc != nil {
		return m.FileExistsFunc(fileID)
	}
	return m.FileExistsResult, m.FileExistsError
}

// DeleteFile implements Storage interface
func (m *MockStorage) DeleteFile(fileID string) error {
	m.DeleteFileCalls = append(m.DeleteFileCalls, fileID)
	if m.DeleteFileFunc != nil {
		return m.DeleteFileFunc(fileID)
	}
	return m.DeleteFileError
}

// DownloadFile implements Storage interface
func (m *MockStorage) DownloadFile(fileID string) (string, error) {
	m.DownloadFileCalls = append(m.DownloadFileCalls, fileID)
	if m.DownloadFileFunc != nil {
		return m.DownloadFileFunc(fileID)
	}
	return m.DownloadFileContent, m.DownloadFileError
}

// DownloadFileToTemp implements Storage interface
func (m *MockStorage) DownloadFileToTemp(fileID string) (string, error) {
	m.DownloadFileToTempCalls = append(m.DownloadFileToTempCalls, fileID)
	if m.DownloadFileToTempFunc != nil {
		return m.DownloadFileToTempFunc(fileID)
	}
	return m.DownloadFileToTempPath, m.DownloadFileToTempError
}

// UploadFile implements Storage interface
func (m *MockStorage) UploadFile(filePath, filename, mimeType string) (string, error) {
	m.UploadFileCalls = append(m.UploadFileCalls, UploadFileCall{
		FilePath: filePath,
		Filename: filename,
		MimeType: mimeType,
	})
	if m.UploadFileFunc != nil {
		return m.UploadFileFunc(filePath, filename, mimeType)
	}
	return m.UploadFileID, m.UploadFileError
}

// UploadString implements Storage interface
func (m *MockStorage) UploadString(content, filename, mimeType, fileID string) (string, error) {
	m.UploadStringCalls = append(m.UploadStringCalls, UploadStringCall{
		Content:  content,
		Filename: filename,
		MimeType: mimeType,
		FileID:   fileID,
	})
	if m.UploadStringFunc != nil {
		return m.UploadStringFunc(content, filename, mimeType, fileID)
	}
	return m.UploadStringID, m.UploadStringError
}

// Reset clears all call tracking and resets the mock to default state.
func (m *MockStorage) Reset() {
	// Clear function overrides
	m.GenerateDownloadURLFunc = nil
	m.ExtractFileIDFromURLFunc = nil
	m.GetFilesFunc = nil
	m.GetMostRecentFileFunc = nil
	m.FileExistsFunc = nil
	m.DeleteFileFunc = nil
	m.DownloadFileFunc = nil
	m.DownloadFileToTempFunc = nil
	m.UploadFileFunc = nil
	m.UploadStringFunc = nil

	// Clear simple return values
	m.GetFilesError = nil
	m.GetFilesFiles = nil
	m.GetMostRecentFileFile = nil
	m.FileExistsResult = false
	m.FileExistsError = nil
	m.DeleteFileError = nil
	m.DownloadFileContent = ""
	m.DownloadFileError = nil
	m.DownloadFileToTempPath = ""
	m.DownloadFileToTempError = nil
	m.UploadFileID = ""
	m.UploadFileError = nil
	m.UploadStringID = ""
	m.UploadStringError = nil

	// Clear call tracking
	m.GenerateDownloadURLCalls = make([]string, 0)
	m.ExtractFileIDFromURLCalls = make([]string, 0)
	m.GetFilesCalls = make([]GetFilesCall, 0)
	m.GetMostRecentFileCalls = make([][]*drive.File, 0)
	m.FileExistsCalls = make([]string, 0)
	m.DeleteFileCalls = make([]string, 0)
	m.DownloadFileCalls = make([]string, 0)
	m.DownloadFileToTempCalls = make([]string, 0)
	m.UploadFileCalls = make([]UploadFileCall, 0)
	m.UploadStringCalls = make([]UploadStringCall, 0)
}

// CallCount returns the number of calls made to each method for verification.
func (m *MockStorage) CallCount() map[string]int {
	return map[string]int{
		"GenerateDownloadURL":  len(m.GenerateDownloadURLCalls),
		"ExtractFileIDFromURL": len(m.ExtractFileIDFromURLCalls),
		"GetFiles":             len(m.GetFilesCalls),
		"GetMostRecentFile":    len(m.GetMostRecentFileCalls),
		"FileExists":           len(m.FileExistsCalls),
		"DeleteFile":           len(m.DeleteFileCalls),
		"DownloadFile":         len(m.DownloadFileCalls),
		"DownloadFileToTemp":   len(m.DownloadFileToTempCalls),
		"UploadFile":           len(m.UploadFileCalls),
		"UploadString":         len(m.UploadStringCalls),
	}
}
