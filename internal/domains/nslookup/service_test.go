package nslookup_test

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/Fivegen-LLC/sdwan-lib/pkg/config"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/nslookup"
	"github.com/Fivegen-LLC/sdwan-agent/internal/domains/nslookup/nslookup_mocks"
)

var (
	errTestError = errors.New("test error")
)

const (
	testFQDN        = "test.sdwan.com"
	testReserveFQDN = "test.reserve.sdwan.com"
	testHostsPrefix = "test_hosts"
)

type serviceFields struct {
	configService   *nslookup_mocks.IConfigService
	lookupIPService *nslookup_mocks.ILookupIPService
}

func newServiceFields(t *testing.T) *serviceFields {
	return &serviceFields{
		configService:   nslookup_mocks.NewIConfigService(t),
		lookupIPService: nslookup_mocks.NewILookupIPService(t),
	}
}

func TestService_SyncHosts(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		name                 string
		existingHostsContent string
		expectedHostsContent string
		skipCreateFile       bool
		prepare              func(f *serviceFields)
		expectedErr          error
	}{
		{
			name:           "empty config",
			skipCreateFile: true,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(config.Config{}, nil)
			},
		},
		{
			name:           "empty orchestrator address",
			skipCreateFile: true,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{},
						}, nil,
					)
			},
		},
		{
			name: "create new hosts",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)
			},
		},
		{
			name: "create new multiple hosts",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
					net.ParseIP("192.168.80.90"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)
			},
		},
		{
			name: "append not existing hosts",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
192.168.132.12 test.sdwan.com
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
					net.ParseIP("192.168.80.90"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)
			},
		},
		{
			name: "delete not existing hosts",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)
			},
		},
		{
			name: "skip appending hosts",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
					net.ParseIP("192.168.80.90"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)
			},
		},
		{
			name: "create new multiple hosts for multi orchestrator",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
172.20.180.10 test.reserve.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{
									testFQDN,
									testReserveFQDN,
								},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
					net.ParseIP("192.168.80.90"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)

				ipsReserve := []net.IP{
					net.ParseIP("172.20.180.10"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testReserveFQDN).
					Return(ipsReserve, nil)
			},
		},
		{
			name: "append not existing hosts for multi orchestrator",
			existingHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309
172.20.180.10 test.reserve.sdwan.com
192.168.132.12 test.sdwan.com

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
`,
			expectedHostsContent: `127.0.0.1 localhost
127.0.1.1 C34377020720251309

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix

# SDWAN: generated section
192.168.132.12 test.sdwan.com
192.168.80.90 test.sdwan.com
172.20.180.10 test.reserve.sdwan.com
172.210.20.10 test.reserve.sdwan.com
`,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{
									testFQDN,
									testReserveFQDN,
								},
							},
						}, nil,
					)

				ips := []net.IP{
					net.ParseIP("192.168.132.12"),
					net.ParseIP("192.168.80.90"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(ips, nil)

				ipsReserve := []net.IP{
					net.ParseIP("172.20.180.10"),
					net.ParseIP("172.210.20.10"),
				}
				f.lookupIPService.EXPECT().
					LookupIP(testReserveFQDN).
					Return(ipsReserve, nil)
			},
		},
		{
			name:           "get config error",
			skipCreateFile: true,
			expectedErr:    errTestError,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{}, errTestError,
					)
			},
		},
		{
			name:           "lookup IPs error",
			skipCreateFile: true,
			expectedErr:    errTestError,
			prepare: func(f *serviceFields) {
				f.configService.EXPECT().
					GetConfig().
					Return(
						config.Config{
							App: &config.AppSection{
								OrchestratorAddrs: []string{testFQDN},
							},
						}, nil,
					)

				f.lookupIPService.EXPECT().
					LookupIP(testFQDN).
					Return(nil, errTestError)
			},
		},
	}

	for i, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			f := newServiceFields(t)
			if testCase.prepare != nil {
				testCase.prepare(f)
			}

			// create hosts file
			etcHostsFile := fmt.Sprintf("%s_%d", testHostsPrefix, i+1)
			if !testCase.skipCreateFile {
				err := os.WriteFile(etcHostsFile, []byte(testCase.existingHostsContent), 0600)
				require.NoError(t, err)

				defer func() {
					if err = os.Remove(etcHostsFile); err != nil {
						log.Fatal().Err(err).Msg("delete test /etc/hosts file error")
					}
				}()
			}

			service := nslookup.NewService(
				f.configService,
				f.lookupIPService,
				etcHostsFile,
			)

			err := service.SyncHosts()
			if err != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, testCase.expectedErr)
				return
			}

			require.NoError(t, err)
			if testCase.skipCreateFile {
				return
			}

			data, err := os.ReadFile(etcHostsFile)
			require.NoError(t, err)
			assert.Equal(t, testCase.expectedHostsContent, string(data))
		})
	}
}
