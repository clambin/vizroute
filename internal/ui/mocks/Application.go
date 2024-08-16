// Code generated by mockery v2.44.1. DO NOT EDIT.

package mocks

import (
	tview "github.com/rivo/tview"
	mock "github.com/stretchr/testify/mock"
)

// Application is an autogenerated mock type for the Application type
type Application struct {
	mock.Mock
}

type Application_Expecter struct {
	mock *mock.Mock
}

func (_m *Application) EXPECT() *Application_Expecter {
	return &Application_Expecter{mock: &_m.Mock}
}

// QueueUpdateDraw provides a mock function with given fields: _a0
func (_m *Application) QueueUpdateDraw(_a0 func()) *tview.Application {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for QueueUpdateDraw")
	}

	var r0 *tview.Application
	if rf, ok := ret.Get(0).(func(func()) *tview.Application); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*tview.Application)
		}
	}

	return r0
}

// Application_QueueUpdateDraw_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'QueueUpdateDraw'
type Application_QueueUpdateDraw_Call struct {
	*mock.Call
}

// QueueUpdateDraw is a helper method to define mock.On call
//   - _a0 func()
func (_e *Application_Expecter) QueueUpdateDraw(_a0 interface{}) *Application_QueueUpdateDraw_Call {
	return &Application_QueueUpdateDraw_Call{Call: _e.mock.On("QueueUpdateDraw", _a0)}
}

func (_c *Application_QueueUpdateDraw_Call) Run(run func(_a0 func())) *Application_QueueUpdateDraw_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(func()))
	})
	return _c
}

func (_c *Application_QueueUpdateDraw_Call) Return(_a0 *tview.Application) *Application_QueueUpdateDraw_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Application_QueueUpdateDraw_Call) RunAndReturn(run func(func()) *tview.Application) *Application_QueueUpdateDraw_Call {
	_c.Call.Return(run)
	return _c
}

// NewApplication creates a new instance of Application. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewApplication(t interface {
	mock.TestingT
	Cleanup(func())
}) *Application {
	mock := &Application{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
