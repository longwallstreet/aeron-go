/*
Copyright 2016 Stanislav Liberman

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aeron

import (
	"github.com/lirm/aeron-go/aeron/atomic"
	"github.com/lirm/aeron-go/aeron/logbuffer/term"
)

type Subscription struct {
	conductor       *ClientConductor
	channel         string
	roundRobinIndex int
	registrationId  int64
	streamId        int32

	images *ImageList

	isClosed atomic.Bool
}

func NewSubscription(conductor *ClientConductor, channel string, registrationId int64, streamId int32) *Subscription {
	sub := new(Subscription)
	sub.images = NewImageList()
	sub.conductor = conductor
	sub.channel = channel
	sub.registrationId = registrationId
	sub.streamId = streamId
	sub.roundRobinIndex = 0
	sub.isClosed.Set(false)

	return sub
}

func (sub *Subscription) IsClosed() bool {
	return sub.isClosed.Get()
}

func (sub *Subscription) Close() error {
	if sub.isClosed.CompareAndSet(false, true) {
		<-sub.conductor.releaseSubscription(sub.registrationId, sub.images.Get())
	}

	return nil
}

func (sub *Subscription) Poll(handler term.FragmentHandler, fragmentLimit int) int {

	img := sub.images.Get()
	length := len(img)
	var fragmentsRead int = 0

	if length > 0 {
		var startingIndex int = sub.roundRobinIndex
		sub.roundRobinIndex++
		if startingIndex >= length {
			sub.roundRobinIndex = 0
			startingIndex = 0
		}

		for i := startingIndex; i < length && fragmentsRead < fragmentLimit; i++ {
			fragmentsRead += img[i].Poll(handler, fragmentLimit-fragmentsRead)
		}

		for i := 0; i < startingIndex && fragmentsRead < fragmentLimit; i++ {
			fragmentsRead += img[i].Poll(handler, fragmentLimit-fragmentsRead)
		}
	}

	return fragmentsRead
}

func (sub *Subscription) hasImage(sessionId int32) bool {
	img := sub.images.Get()
	for _, image := range img {
		if image.sessionId == sessionId {
			return true
		}
	}
	return false
}

func (sub *Subscription) addImage(image *Image) *[]*Image {

	images := sub.images.Get()

	sub.images.Set(append(images, image))

	return &images
}

func (sub *Subscription) removeImage(correlationId int64) *Image {

	img := sub.images.Get()
	for ix, image := range img {
		if image.correlationId == correlationId {
			logger.Debugf("Removing image %v for subscription %d", image, sub.registrationId)

			img[ix] = img[len(img)-1]
			img[len(img)-1] = nil
			img = img[:len(img)-1]

			// FIXME CAS to make sure it's the same list
			sub.images.Set(img)

			return image
		}
	}
	return nil
}

func (sub *Subscription) HasImages() bool {
	images := sub.images.Get()
	return len(images) > 0
}

func IsConnectedTo(sub *Subscription, pub *Publication) bool {
	img := sub.images.Get()
	if sub.channel == pub.channel && sub.streamId == pub.streamId {
		for _, image := range img {
			if image.sessionId == pub.sessionId {
				return true
			}
		}
	}

	return false
}
