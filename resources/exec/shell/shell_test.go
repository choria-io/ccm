// Copyright (c) 2025-2026, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/choria-io/ccm/model"
	"github.com/choria-io/ccm/model/modelmocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
)

var _ = Describe("Shell Provider", func() {
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

		provider, err = NewShellProvider(logger, runner)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		mockctl.Finish()
	})

	Describe("NewShellProvider", func() {
		It("Should create a provider with the given logger and runner", func() {
			p, err := NewShellProvider(logger, runner)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.log).To(Equal(logger))
			Expect(p.runner).To(Equal(runner))
		})

		It("Should create a provider with nil runner", func() {
			p, err := NewShellProvider(logger, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(p).ToNot(BeNil())
			Expect(p.runner).To(BeNil())
		})
	})

	Describe("Name", func() {
		It("Should return the provider name", func() {
			Expect(provider.Name()).To(Equal("shell"))
		})
	})

	Describe("Execute", func() {
		It("Should execute a simple command via sh -c", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "echo-hello",
				},
				Command: "echo hello",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "echo hello"},
			}).Return([]byte("hello\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should execute a command with pipes", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "piped-command",
				},
				Command: "echo hello | grep hello",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "echo hello | grep hello"},
			}).Return([]byte("hello\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should execute a command with shell builtins", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "builtin-command",
				},
				Command: "for i in 1 2 3; do echo $i; done",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "for i in 1 2 3; do echo $i; done"},
			}).Return([]byte("1\n2\n3\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should execute a command with redirections", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "redirect-command",
				},
				Command: "echo hello > /tmp/test.txt && cat /tmp/test.txt",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "echo hello > /tmp/test.txt && cat /tmp/test.txt"},
			}).Return([]byte("hello\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Cwd to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "cwd-command",
				},
				Command: "pwd",
				Cwd:     "/tmp",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "pwd"},
				Cwd:     "/tmp",
			}).Return([]byte("/tmp\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Environment to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "env-command",
				},
				Command:     "echo $FOO $BAZ",
				Environment: []string{"FOO=bar", "BAZ=qux"},
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command:     "/bin/sh",
				Args:        []string{"-c", "echo $FOO $BAZ"},
				Environment: []string{"FOO=bar", "BAZ=qux"},
			}).Return([]byte("bar qux\n"), []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass Path to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "path-command",
				},
				Command: "mycommand",
				Path:    "/usr/local/bin:/usr/bin:/bin",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "mycommand"},
				Path:    "/usr/local/bin:/usr/bin:/bin",
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should pass ParsedTimeout to the runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "timeout-command",
				},
				Command:       "sleep 1",
				ParsedTimeout: 30 * time.Second,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "sleep 1"},
				Timeout: 30 * time.Second,
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		It("Should return non-zero exit code on command failure", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "failing-command",
				},
				Command: "exit 1",
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command: "/bin/sh",
				Args:    []string{"-c", "exit 1"},
			}).Return([]byte{}, []byte{}, 1, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(1))
		})

		It("Should return error from runner", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "error-command",
				},
				Command: "some command",
			}

			expectedErr := errors.New("runner error")
			runner.EXPECT().ExecuteWithOptions(gomock.Any(), gomock.Any()).Return(nil, nil, -1, expectedErr)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(expectedErr))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should return error when runner is nil", func() {
			provider, err := NewShellProvider(logger, nil)
			Expect(err).ToNot(HaveOccurred())

			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "nil-runner-command",
				},
				Command: "echo hello",
			}

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no command runner configured"))
			Expect(exitCode).To(Equal(-1))
		})

		It("Should pass all options together", func() {
			properties := &model.ExecResourceProperties{
				CommonResourceProperties: model.CommonResourceProperties{
					Name: "all-options-command",
				},
				Command:       "echo $VAR1 | tee output.txt",
				Cwd:           "/home/user",
				Environment:   []string{"VAR1=value1"},
				Path:          "/custom/path",
				ParsedTimeout: 60 * time.Second,
			}

			runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
				Command:     "/bin/sh",
				Args:        []string{"-c", "echo $VAR1 | tee output.txt"},
				Cwd:         "/home/user",
				Environment: []string{"VAR1=value1"},
				Path:        "/custom/path",
				Timeout:     60 * time.Second,
			}).Return([]byte{}, []byte{}, 0, nil)

			exitCode, err := provider.Execute(context.Background(), properties, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(exitCode).To(Equal(0))
		})

		Context("with LogOutput", func() {
			It("Should log output when LogOutput is true and logger is provided", func() {
				userLogger := modelmocks.NewMockLogger(mockctl)
				userLogger.EXPECT().Info("hello world").Times(1)

				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "log-output-command",
					},
					Command:   "echo 'hello world'",
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/sh",
					Args:    []string{"-c", "echo 'hello world'"},
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
						Name: "multiline-command",
					},
					Command:   "printf 'line one\nline two\nline three\n'",
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
						Name: "no-log-command",
					},
					Command:   "echo hello",
					LogOutput: false,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/sh",
					Args:    []string{"-c", "echo hello"},
				}).Return([]byte("hello\n"), []byte{}, 0, nil)

				exitCode, err := provider.Execute(context.Background(), properties, userLogger)
				Expect(err).ToNot(HaveOccurred())
				Expect(exitCode).To(Equal(0))
			})

			It("Should not log output when logger is nil even if LogOutput is true", func() {
				properties := &model.ExecResourceProperties{
					CommonResourceProperties: model.CommonResourceProperties{
						Name: "nil-logger-command",
					},
					Command:   "echo hello",
					LogOutput: true,
				}

				runner.EXPECT().ExecuteWithOptions(gomock.Any(), model.ExtendedExecOptions{
					Command: "/bin/sh",
					Args:    []string{"-c", "echo hello"},
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
						Name: "empty-lines-command",
					},
					Command:   "printf 'first\n\nsecond\n'",
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
					Name: "creates-exists",
				},
				Command: "touch marker",
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
					Name: "creates-missing",
				},
				Command: "touch marker",
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
					Name: "creates-empty",
				},
				Command: "echo hello",
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
					Name: "creates-directory",
				},
				Command: "mkdir -p /some/dir",
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
			Expect(ProviderName).To(Equal("shell"))
		})
	})
})
