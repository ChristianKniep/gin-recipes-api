package handlers

import "github.com/opentracing/opentracing-go"

func NewSubSpan(sp opentracing.Span, name string) opentracing.Span {
	return opentracing.StartSpan(name, opentracing.ChildOf(sp.Context()))
}
