LOCAL_PATH := $(call my-dir)

include $(CLEAR_VARS)

LOCAL_MODULE := touchtest

LOCAL_SRC_FILES := kmods.cpp \
                   TouchInput.cpp \

LOCAL_CFLAGS += -O0 -g
LOCAL_CPPFLAGS += -O0 -g

LOCAL_LDLIBS += -llog

include $(BUILD_EXECUTABLE)