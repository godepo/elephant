// Code generated by mockery v2.53.3. DO NOT EDIT.

package collector

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockInterceptor is an autogenerated mock type for the Interceptor type
type MockInterceptor struct {
	mock.Mock
}

type MockInterceptor_Expecter struct {
	mock *mock.Mock
}

func (_m *MockInterceptor) EXPECT() *MockInterceptor_Expecter {
	return &MockInterceptor_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: ctx, err
func (_m *MockInterceptor) Execute(ctx context.Context, err error) string {
	ret := _m.Called(ctx, err)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func(context.Context, error) string); ok {
		r0 = rf(ctx, err)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockInterceptor_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MockInterceptor_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - ctx context.Context
//   - err error
func (_e *MockInterceptor_Expecter) Execute(ctx interface{}, err interface{}) *MockInterceptor_Execute_Call {
	return &MockInterceptor_Execute_Call{Call: _e.mock.On("Execute", ctx, err)}
}

func (_c *MockInterceptor_Execute_Call) Run(run func(ctx context.Context, err error)) *MockInterceptor_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(error))
	})
	return _c
}

func (_c *MockInterceptor_Execute_Call) Return(_a0 string) *MockInterceptor_Execute_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockInterceptor_Execute_Call) RunAndReturn(run func(context.Context, error) string) *MockInterceptor_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockInterceptor creates a new instance of MockInterceptor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockInterceptor(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInterceptor {
	mock := &MockInterceptor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
