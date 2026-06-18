package queryexec

import (
	"context"
	"errors"
)

type PipelineOptions struct {
	Limit int
}

type Pipeline struct {
	source  Operator
	options PipelineOptions
	profile Profile
	err     error
	read    int
	closed  bool
}

func NewPipeline(source Operator, options PipelineOptions) *Pipeline {
	profile := Profile{Operators: []OperatorProfile{}}
	if source != nil {
		profile.Operators = append(profile.Operators, OperatorProfile{ID: source.ID()})
	}
	return &Pipeline{
		source:  source,
		options: options,
		profile: profile,
	}
}

func (p *Pipeline) Next(ctx context.Context) bool {
	if p.closed || p.source == nil || p.limitReached() {
		p.err = errors.Join(p.err, p.closeSource())
		return false
	}
	if err := ctx.Err(); err != nil {
		p.err = err
		p.err = errors.Join(p.err, p.closeSource())
		return false
	}
	_, ok := p.source.Next(ctx)
	if !ok {
		p.err = p.source.Err()
		p.recordError()
		p.err = errors.Join(p.err, p.closeSource())
		return false
	}
	p.read++
	p.profile.Operators[0].RowsOut = p.read
	if p.limitReached() {
		p.err = errors.Join(p.err, p.closeSource())
	}
	return true
}

func (p *Pipeline) Err() error {
	return p.err
}

func (p *Pipeline) Profile() Profile {
	return p.profile
}

func (p *Pipeline) Close() error {
	return p.closeSource()
}

func (p *Pipeline) limitReached() bool {
	return p.options.Limit > 0 && p.read >= p.options.Limit
}

func (p *Pipeline) closeSource() error {
	if p.closed || p.source == nil {
		return nil
	}
	p.closed = true
	return p.source.Close()
}

func (p *Pipeline) recordError() {
	if p.err == nil || len(p.profile.Operators) == 0 {
		return
	}
	p.profile.Operators[0].Error = p.err.Error()
}
