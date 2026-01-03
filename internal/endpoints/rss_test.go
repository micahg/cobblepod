package endpoints

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"cobblepod/internal/storage"
	storagemock "cobblepod/internal/storage/mock"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRSSProcessor implements RSSProcessor
type MockRSSProcessor struct {
	mock.Mock
}

func (m *MockRSSProcessor) GetRSSFeedID(ctx context.Context) string {
	args := m.Called(ctx)
	return args.String(0)
}

func TestHandleGetRSS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMocks     func(*storagemock.MockStorage, *MockRSSProcessor)
		setupAuth      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Success",
			setupMocks: func(ms *storagemock.MockStorage, mp *MockRSSProcessor) {
				mp.On("GetRSSFeedID", mock.Anything).Return("test-file-id")
				ms.GenerateDownloadURLFunc = func(driveID string) string {
					return "http://download.url"
				}
			},
			setupAuth: func() {
				getGoogleAccessTokenFunc = func(ctx context.Context, userID string) (string, error) {
					return "valid-token", nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"url":"http://download.url"}`,
		},
		{
			name: "RSS Feed Not Found",
			setupMocks: func(ms *storagemock.MockStorage, mp *MockRSSProcessor) {
				mp.On("GetRSSFeedID", mock.Anything).Return("")
			},
			setupAuth: func() {
				getGoogleAccessTokenFunc = func(ctx context.Context, userID string) (string, error) {
					return "valid-token", nil
				}
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `{"error":"RSS feed not found. Please process a backup first."}`,
		},
		{
			name:       "Google Auth Error",
			setupMocks: func(ms *storagemock.MockStorage, mp *MockRSSProcessor) {},
			setupAuth: func() {
				getGoogleAccessTokenFunc = func(ctx context.Context, userID string) (string, error) {
					return "", errors.New("auth error")
				}
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   `{"error":"Failed to authenticate with Google: auth error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockStorage := storagemock.NewMockStorage()
			mockProcessor := new(MockRSSProcessor)

			// Save original functions and restore after test
			origGetToken := getGoogleAccessTokenFunc
			origNewStorage := newStorageServiceFunc
			origNewProcessor := newRSSProcessorFunc
			defer func() {
				getGoogleAccessTokenFunc = origGetToken
				newStorageServiceFunc = origNewStorage
				newRSSProcessorFunc = origNewProcessor
			}()

			// Setup dependency injection
			newStorageServiceFunc = func(ctx context.Context, token string) (storage.Storage, error) {
				return mockStorage, nil
			}
			newRSSProcessorFunc = func(channelTitle string, driveService storage.Storage) RSSProcessor {
				return mockProcessor
			}

			if tt.setupAuth != nil {
				tt.setupAuth()
			}
			if tt.setupMocks != nil {
				tt.setupMocks(mockStorage, mockProcessor)
			}

			// Setup router
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("user_id", "test-user")
				c.Next()
			})
			r.GET("/rss", HandleGetRSS())

			// Perform request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/rss", nil)
			r.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != "" {
				assert.JSONEq(t, tt.expectedBody, w.Body.String())
			}

			// mockStorage verification is manual if needed, but we just check behavior here
			mockProcessor.AssertExpectations(t)
		})
	}
}
