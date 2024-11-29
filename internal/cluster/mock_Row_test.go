// Code generated by mockery v2.40.3. DO NOT EDIT.

package cluster

import mock "github.com/stretchr/testify/mock"

// MockRow is an autogenerated mock type for the Row type
type MockRow struct {
	mock.Mock
}

type MockRow_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRow) EXPECT() *MockRow_Expecter {
	return &MockRow_Expecter{mock: &_m.Mock}
}

// Scan provides a mock function with given fields: dest
func (_m *MockRow) Scan(dest ...interface{}) error {
	var _ca []interface{}
	_ca = append(_ca, dest...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Scan")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(...interface{}) error); ok {
		r0 = rf(dest...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRow_Scan_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Scan'
type MockRow_Scan_Call struct {
	*mock.Call
}

// Scan is a helper method to define mock.On call
//   - dest ...interface{}
func (_e *MockRow_Expecter) Scan(dest ...interface{}) *MockRow_Scan_Call {
	return &MockRow_Scan_Call{Call: _e.mock.On("Scan",
		append([]interface{}{}, dest...)...)}
}

func (_c *MockRow_Scan_Call) Run(run func(dest ...interface{})) *MockRow_Scan_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]interface{}, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockRow_Scan_Call) Return(_a0 error) *MockRow_Scan_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRow_Scan_Call) RunAndReturn(run func(...interface{}) error) *MockRow_Scan_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockRow creates a new instance of MockRow. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRow(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRow {
	mock := &MockRow{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
