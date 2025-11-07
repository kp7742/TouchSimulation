#ifndef TOUCHINPUT_H
#define TOUCHINPUT_H

#include <ctime>
#include <cstdio>
#include <dirent.h>
#include <cstring>
#include <cstdlib>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <linux/uinput.h>

#include <vector>
#include <thread>

using namespace std;

namespace TouchInput {
    ///Touch Contact Struct
    struct TouchContact {
        int posX = -1;
        int posY = -1;
        bool enabled = false;
    };

    struct InputDevice {
        int ifd;
        int slots;
        int version;
        int touchXMin;
        int touchXMax;
        int touchYMin;
        int touchYMax;
        bool isGrabbed;
        bool hasPressure;
        bool hasTouchMajor;
        bool hasTouchMinor;
        bool hasWidthMajor;
        bool hasWidthMinor;
        bool hasOrientation;
        struct input_id iid;
        unsigned char evbits[EV_MAX / 8 + 1];
        unsigned char absBits[ABS_MAX / 8 + 1];
        unsigned char keyBits[KEY_MAX / 8 + 1];
        unsigned char propBits[INPUT_PROP_MAX / 8 + 1];
        string name;
        string path;
        string phys;
        vector<struct input_absinfo> absInfos;
    };

    void sendTouchMove(int x, int y);

    void sendTouchUp();

    void updateRes(int x, int y);

    void touchInputStart();

    void touchInputStop();

    void Test();
}

#endif //TOUCHINPUT_H