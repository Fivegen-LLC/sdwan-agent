package discovery_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/discovery"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/discovery/discovery_mocks"
	"github.com/Fivegen-LLC/sdwan-agent/internal/errs"
)

type serviceFields struct {
	httpClientService *discovery_mocks.MockIHTTPClientService
}

func newServiceFields(t *testing.T) *serviceFields {
	return &serviceFields{
		httpClientService: discovery_mocks.NewMockIHTTPClientService(t),
	}
}

// parallel tests isn't recommended because http server is started on the same ports for different test cases.
func Test_FetchPrimary(t *testing.T) {
	testTable := []struct {
		name            string
		prepare         func(f *serviceFields)
		expectedPrimary string
		expectedError   error
	}{
		{
			name:            "fetch only one primary",
			expectedPrimary: "orch2.sdwan.lab",
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(true, nil).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(false, nil).
					Times(1)
			},
			expectedError: nil,
		},
		{
			name:          "split brain error",
			expectedError: errs.ErrSplitBrain,
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(true, nil).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(true, nil).
					Times(1)
			},
		},
		{
			name:          "primary is down",
			expectedError: errs.ErrPrimaryNotFound,
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(false, errors.New("network error")).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(false, nil).
					Times(1)
			},
		},
		{
			name:            "standby is down",
			expectedPrimary: "orch2.sdwan.lab",
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(true, nil).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(false, errors.New("network error")).
					Times(1)
			},
			expectedError: nil,
		},
		{
			name: "two hosts down",
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(false, errors.New("network error")).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(false, errors.New("network error")).
					Times(1)
			},
			expectedError: errs.ErrPrimaryNotFound,
		},
		{
			name: "primary not found",
			prepare: func(f *serviceFields) {
				f.httpClientService.EXPECT().
					CheckPrimary("orch2.sdwan.lab").
					Return(false, nil).
					Times(1)

				f.httpClientService.EXPECT().
					CheckPrimary("orch22.sdwan.lab").
					Return(false, nil).
					Times(1)
			},
			expectedError: errs.ErrPrimaryNotFound,
		},
	}
	for _, testCase := range testTable {
		t.Run(testCase.name, func(_ *testing.T) {
			f := newServiceFields(t)
			if testCase.prepare != nil {
				testCase.prepare(f)
			}

			service := discovery.NewService(f.httpClientService)

			host, err := service.FetchPrimary([]string{
				"orch2.sdwan.lab",
				"orch22.sdwan.lab",
			})
			if testCase.expectedError != nil {
				require.ErrorIs(t, err, testCase.expectedError)
			} else {
				require.Equal(t, testCase.expectedPrimary, host)
			}

		})
	}
}
