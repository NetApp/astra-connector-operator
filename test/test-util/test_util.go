// Copyright 2023 NetApp, Inc. All Rights Reserved.

package test_util

import (
	"fmt"
	stdlog "log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	astrav1 "github.com/NetApp-Polaris/astra-connector-operator/details/operator-sdk/api/v1"
)

// CreateLoggerForTesting reates a golang standard logging implementation of logr.Logger. This allows tests to still log output for easier debug
// Also set the prefix for the logger to be the name of the Test. This should make debugging better when there is a long list of log output
// on e.g. Jenkins logs.
// Optionally pass a testing.T pointer and the actual test name will be used for the prefix, else we will attempt to
// find the test name in the call stack.
func CreateLoggerForTesting(t ...*testing.T) logr.Logger {
	var testName string
	if len(t) > 0 {
		testName = t[0].Name()
	} else {
		testName, _ = FindTestName(true)
	}
	prefix := testName + ": "
	log := stdr.New(stdlog.New(os.Stderr, prefix, stdlog.LstdFlags|stdlog.Lmicroseconds|stdlog.Lshortfile))

	logLevel, err := strconv.Atoi(os.Getenv("ACC_TEST_LOG_LEVEL"))
	if err != nil {
		logLevel = 10
	}
	stdr.SetVerbosity(logLevel)

	return log
}

// FindTestName searches the call stack for the first caller that matches go test requirements of file that ends with "_go.test", and
// a Function that starts with "Test".
func FindTestName(includeFullPkgPath bool) (string, error) {
	// log := stdr.New(stdlog.New(os.Stderr, "FindTestName: ", stdlog.LstdFlags)) // For use in debugging this method
	skip := 1 // Start at 1, because we know this method is not the Test Method
	for true {
		pc, file, _, ok := runtime.Caller(skip)
		if ok {
			// log.Info("", "skip", skip, "file", file, "name", runtime.FuncForPC(pc).Name())
			if strings.HasSuffix(file, "_test.go") {
				// It is a test file, check to see it the Function name starts with Test
				name := runtime.FuncForPC(pc).Name()
				parts := strings.Split(name, "/")
				funcName := parts[len(parts)-1]
				parts2 := strings.Split(funcName, ".")
				for _, part := range parts2 {
					if strings.HasPrefix(part, "Test") {
						if includeFullPkgPath {
							return funcName, nil
						} else {
							return part, nil
						}
					}
				}
			}
		} else {
			break
		}
		skip++
	}
	return "TestNameNotFound", fmt.Errorf("no Caller matching expected test file/function names found")
}

func CreateFakeClient(initObjs ...client.Object) client.Client {
	ac := &astrav1.AstraConnector{}

	// Create a fake k8s api client
	s := clientgoscheme.Scheme
	s.AddKnownTypes(astrav1.SchemeBuilder.GroupVersion, ac)

	cb := fake.NewClientBuilder()
	cb = cb.WithScheme(s)
	cb = cb.WithObjects(initObjs...)
	return cb.Build()
}
