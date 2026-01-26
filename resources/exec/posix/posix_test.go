// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package posix

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
)

func TestPosixProvider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources/Exec/Posix")
}

var _ = Describe("Posix Provider", func() {
	var (
		mockctl  *gomock.Controller
		logger   *modelmocks.MockLogger
		runner   *modelmocks.MockCommandRunner
		provider *Provider
		err      error
	)

	BeforeEach(func() {
		mockctl = gomock.NewController(GinkgoT())
		logger = modelmocks.NewMockLogger(mockctl)
		runner = modelmocks.NewMockCommandRunner(mockctl)

		logger.EXPECT().Info(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Debug(gomock.Any(), gomock.Any()).AnyTimes()
		logger.EXPECT().Warn(gomock.Any(), gomock.Any()).AnyTimes()

		provider, err = NewPosixProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("NewPosixProvider", func() {
		It("Should create a provider with the given logger and runner", func() {
			p, err := NewPosixProvider(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(logger))
			Expect(p.runner).To(Equal(runner))
		})

		It("Should create a provider with nil runner", func() {
			p, err := NewPosixProvider(logger, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.runner).To(BeNil())
		})
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("posix"))
		})
	})

	Describe("Execute", func() {
		It("Should execute a simple command", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/echo",
				Args:    []string{"hello"},
			}).Return([]byte("hello\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should execute a command without arguments", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/pwd",
				},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/pwd",
				Args:    nil,
			}).Return([]byte("/tmp\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Cwd to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/pwd",
				},
				Cwd: "/tmp",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/pwd",
				Args:    nil,
				Cwd:     "/tmp",
			}).Return([]byte("/tmp\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Environment to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/env",
				},
				Environment: []string{"FOO=bar", "BAZ=qux"},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command:     "/bin/env",
				Args:        nil,
				Environment: []string{"FOO=bar", "BAZ=qux"},
			}).Return([]byte("FOO=bar\nBAZ=qux\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Path to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "echo hello",
				},
				Path: "/usr/local/bin:/usr/bin:/bin",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "echo",
				Args:    []string{"hello"},
				Path:    "/usr/local/bin:/usr/bin:/bin",
			}).Return([]byte("hello\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass ParsedTimeout to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/sleep 1",
				},
				ParsedTimeout: 30 * time.Second,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sleep",
				Args:    []string{"1"},
				Timeout: 30 * time.Second,
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should return non-zero exit code on command failure", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/false",
				},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/false",
				Args:    nil,
			}).Return([]byte{}, []byte{}, 1, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(1))
		})

		It("Should return error from runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/nonexistent",
				},
			}

			expectedErr := errors.New("command not found")
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Return(nil, nil, -1, expectedErr)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedErr))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should return error for invalid shell quote in command", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo 'unterminated",
				},
			}

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Unterminated"))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should return error for empty command", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "",
				},
			}

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no command specified"))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should return error when runner is nil", func() {
			provider, err := NewPosixProvider(logger, nil)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
			}

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no command runner configured"))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should handle command with quoted arguments", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: `/bin/echo "hello world"`,
				},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/echo",
				Args:    []string{"hello world"},
			}).Return([]byte("hello world\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should handle command with multiple arguments", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/usr/bin/find /var -name '*.log' -type f",
				},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/usr/bin/find",
				Args:    []string{"/var", "-name", "*.log", "-type", "f"},
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass all options together", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/mycommand arg1 arg2",
				},
				Cwd:           "/home/user",
				Environment:   []string{"VAR1=value1"},
				Path:          "/custom/path",
				ParsedTimeout: 60 * time.Second,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command:     "/bin/mycommand",
				Args:        []string{"arg1", "arg2"},
				Cwd:         "/home/user",
				Environment: []string{"VAR1=value1"},
				Path:        "/custom/path",
				Timeout:     60 * time.Second,
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		Context("with Command property", func() {
			It("Should use Command instead of Name when Command is set", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "my-descriptive-name",
					},
					Command: "/bin/echo hello",
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/echo",
					Args:    []string{"hello"},
				}).Return([]byte("hello\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should use Name when Command is empty", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/echo from-name",
					},
					Command: "",
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/echo",
					Args:    []string{"from-name"},
				}).Return([]byte("from-name\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should handle Command with multiple arguments", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "download-binary",
					},
					Command: "/usr/bin/curl -fsSL https://example.com/file.tar.gz -o /tmp/file.tar.gz",
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/usr/bin/curl",
					Args:    []string{"-fsSL", "https://example.com/file.tar.gz", "-o", "/tmp/file.tar.gz"},
				}).Return([]byte{}, []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should handle Command without arguments", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "reload-systemd",
					},
					Command: "/bin/systemctl",
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/systemctl",
					Args:    nil,
				}).Return([]byte{}, []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should return error for invalid shell quote in Command", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "broken-command",
					},
					Command: "/bin/echo 'unterminated",
				}

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Unterminated"))
				Expect(exitCode).To(Equal(-1))
			})

			It("Should pass all options with Command property", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "run-migration",
					},
					Command:       "/opt/app/bin/migrate --run",
					Cwd:           "/opt/app",
					Environment:   []string{"DB_HOST=localhost"},
					Path:          "/opt/app/bin",
					ParsedTimeout: 120 * time.Second,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command:     "/opt/app/bin/migrate",
					Args:        []string{"--run"},
					Cwd:         "/opt/app",
					Environment: []string{"DB_HOST=localhost"},
					Path:        "/opt/app/bin",
					Timeout:     120 * time.Second,
				}).Return([]byte{}, []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})
		})

		Context("with LogOutput", func() {
			It("Should log output when LogOutput is true and logger is provided", func() {
				userLogger := modelmocks.NewMockLogger(mockctl)
				userLogger.EXPECT().Info("hello world").Times(1)

				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/echo hello world",
					},
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/echo",
					Args:    []string{"hello", "world"},
				}).Return([]byte("hello world\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should log each line separately for multi-line output", func() {
				userLogger := modelmocks.NewMockLogger(mockctl)
				gomock.InOrder(
					userLogger.EXPECT().Info("line one"),
					userLogger.EXPECT().Info("line two"),
					userLogger.EXPECT().Info("line three"),
				)

				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/cat multiline",
					},
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Return([]byte("line one\nline two\nline three\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should not log output when LogOutput is false", func() {
				userLogger := modelmocks.NewMockLogger(mockctl)
				// No Info call expected on userLogger

				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/echo hello",
					},
					LogOutput: false,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/echo",
					Args:    []string{"hello"},
				}).Return([]byte("hello\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should not log output when logger is nil even if LogOutput is true", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/echo hello",
					},
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/echo",
					Args:    []string{"hello"},
				}).Return([]byte("hello\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should log empty lines", func() {
				userLogger := modelmocks.NewMockLogger(mockctl)
				gomock.InOrder(
					userLogger.EXPECT().Info("first"),
					userLogger.EXPECT().Info(""),
					userLogger.EXPECT().Info("second"),
				)

				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "/bin/echo lines",
					},
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Return([]byte("first\n\nsecond\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})
		})
	})

	Describe("Status", func() {
		It("Should return CreatesSatisfied=true when creates file exists", func() {
			tmpDir := GinkgoT().TempDir()
			createsFile := filepath.Join(tmpDir, "marker")

			err := os.WriteFile(createsFile, []byte("marker"), 0644)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
				Creates: createsFile,
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.CreatesSatisfied).To(BeTrue())
		})

		It("Should return CreatesSatisfied=false when creates file does not exist", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
				Creates: "/nonexistent/path/marker",
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.CreatesSatisfied).To(BeFalse())
		})

		It("Should return CreatesSatisfied=false when creates is empty", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/echo hello",
				},
				Creates: "",
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.CreatesSatisfied).To(BeFalse())
		})

		It("Should return CreatesSatisfied=true when creates points to a directory", func() {
			tmpDir := GinkgoT().TempDir()

			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "/bin/mkdir /some/dir",
				},
				Creates: tmpDir,
			}

			status, err := provider.Status(context.Background(), properties)
			Expect(err).ToNot(HaveOccurred())
			Expect(status).ToNot(BeNil())
			Expect(status.CreatesSatisfied).To(BeTrue())
		})
	})

	Describe("Constants", func() {
		It("Should have correct provider name", func() {
			Expect(ProviderName).To(Equal("posix"))
		})
	})
})
