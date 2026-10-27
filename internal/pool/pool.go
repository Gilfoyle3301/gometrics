// Package pool содержит generic-контейнер для переиспользования «тяжёлых»
// объектов, состояние которых можно сбросить методом Reset.
package pool

import "sync"

// Resetter — ограничение для типов, хранимых в пуле: объект должен уметь
// сбрасывать своё состояние к начальному.
type Resetter interface {
	Reset()
}

// Pool — потокобезопасный пул объектов одного типа T на основе sync.Pool.
// Перед возвратом объекта в пул метод Put сбрасывает его состояние,
// поэтому Get всегда выдаёт «чистый» объект.
type Pool[T Resetter] struct {
	pool sync.Pool
}

// New создаёт и возвращает указатель на Pool для объектов типа T.
// Фабрика newFunc вызывается sync.Pool, когда свободных объектов нет.
func New[T Resetter](newFunc func() T) *Pool[T] {
	p := &Pool[T]{}
	p.pool.New = func() any {
		return newFunc()
	}
	return p
}

// Get возвращает объект из пула: переиспользуемый, если такой есть,
// иначе созданный фабрикой newFunc.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put сбрасывает состояние объекта через Reset и помещает его в пул
// для повторного использования.
func (p *Pool[T]) Put(v T) {
	v.Reset()
	p.pool.Put(v)
}
