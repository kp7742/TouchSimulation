#ifndef TOUCHINPUT_H
#define TOUCHINPUT_H

#include <time.h>
#include <stdio.h>
#include <dirent.h>
#include <string.h>
#include <stdlib.h>
#include <fcntl.h>
#include <unistd.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <linux/uinput.h>

#include <list>
#include <chrono>
#include <thread>

///Go like Channel Implementation
template<class item>
class Channel {
private:
    std::list<item> queue;
    std::mutex m;
    std::condition_variable cv;
    bool closed;
public:
    Channel() : closed(false) {}

    void close() {
        std::unique_lock<std::mutex> lock(m);
        closed = true;
        cv.notify_all();
    }

    bool is_closed() {
        std::unique_lock<std::mutex> lock(m);
        return closed;
    }

    void put(const item &i) {
        std::unique_lock<std::mutex> lock(m);
        if (closed)
            throw std::logic_error("put to closed channel");
        queue.push_back(i);
        cv.notify_one();
    }

    bool get(item &out, bool wait = true) {
        std::unique_lock<std::mutex> lock(m);
        if (wait)
            cv.wait(lock, [&]() { return closed || !queue.empty(); });
        if (queue.empty())
            return false;
        out = queue.front();
        queue.pop_front();
        return true;
    }
};

namespace TouchInput {
    struct input_absinfo absX;
    struct input_absinfo absY;
    struct input_absinfo absSlot;
    const char letterBytes[] = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ";

    char* randString(int n) {
        char* b = (char*)malloc(n + 1);  // +1 for the null-terminator
        if (b == NULL) {
            perror("Failed to allocate memory");
            exit(1);
        }

        for (int i = 0; i < n; i++) {
            b[i] = letterBytes[rand() % (sizeof(letterBytes) - 1)];
        }
        b[n] = '\0'; // Null-terminate the string

        return b;
    }

    int randNum(int n) {
        // Generate a random number from 0 to n - 1.
        return rand() % n;
    }

    ///Find Touch Device and Create new UInput
    int createUInput(int ifd) {
        int ufd;
        const char *uinput = "/dev/uinput";

        //Get Touch Screen Size
        ioctl(ifd, EVIOCGABS(ABS_MT_SLOT), &absSlot);
        ioctl(ifd, EVIOCGABS(ABS_MT_POSITION_X), &absX);
        ioctl(ifd, EVIOCGABS(ABS_MT_POSITION_Y), &absY);

        //Open UInput
        ufd = open(uinput, O_WRONLY | O_NONBLOCK);
        if (ufd < 0) {
            perror("Unable to open uinput");
        }

        //Setup Touch Value
        ioctl(ufd, UI_SET_EVBIT, EV_KEY);
        ioctl(ufd, UI_SET_KEYBIT, BTN_TOUCH);

        //Setup Touch Params
        ioctl(ufd, UI_SET_EVBIT, EV_ABS);
        ioctl(ufd, UI_SET_ABSBIT, ABS_MT_POSITION_X);
        ioctl(ufd, UI_SET_ABSBIT, ABS_MT_POSITION_Y);
        ioctl(ufd, UI_SET_ABSBIT, ABS_MT_TRACKING_ID);
        ioctl(ufd, UI_SET_PROPBIT, INPUT_PROP_DIRECT);

        //Setup User Device
        struct uinput_user_dev uidev;
        memset(&uidev, 0, sizeof(uidev));
        uidev.id.bustype = 0x1C; //BUS_SPI
        uidev.id.vendor = randNum(0x2000);
        uidev.id.product = randNum(0x2000);
        uidev.id.version = randNum(0x200);

        char* devName = randString(7);
        strncpy(uidev.name, devName, UINPUT_MAX_NAME_SIZE);
        free(devName);

        uidev.absmin[ABS_MT_POSITION_X] = absX.minimum;
        uidev.absmax[ABS_MT_POSITION_X] = absX.maximum;
        uidev.absmin[ABS_MT_POSITION_Y] = absY.minimum;
        uidev.absmax[ABS_MT_POSITION_Y] = absY.maximum;
        uidev.absmin[ABS_MT_TRACKING_ID] = 0;
        uidev.absmax[ABS_MT_TRACKING_ID] = absSlot.maximum;

        //Write to Input Sub-System
        write(ufd, &uidev, sizeof(uidev));

        //Declare Input Device
        ioctl(ufd, UI_DEV_CREATE);

        //Stop Primary Touch Device
        ioctl(ifd, EVIOCGRAB, 1);

        return ufd;
    }

    ///Determine if a event has specified Abs Key.
    bool HasSpecificAbs(int device_fd, unsigned int key) {
        size_t nchar = ABS_MAX / 8 + 1;
        unsigned char bits[nchar];
        ioctl(device_fd, EVIOCGBIT(EV_ABS, sizeof(bits)), &bits);
        return bits[key / 8] & (1 << (key % 8));
    }

    ///Determine if a event has specified Input Prop.
    bool HasSpecificProp(int device_fd, unsigned int prop) {
        size_t nchar = INPUT_PROP_MAX / 8 + 1;
        unsigned char bits[nchar];
        ioctl(device_fd, EVIOCGPROP(sizeof(bits)), &bits);
        return bits[prop / 8] & (1 << (prop % 8));
    }

    ///Determine if a path exist and is a character input device.
    int isCharDevice(const char* path) {
        struct stat st;
        if (stat(path, &st) == -1) {
            return 0; // Not a character device or does not exist
        }

        if (S_ISCHR(st.st_mode)) {
            return 1; // Is a character device
        }

        return 0; // Not a character device
    }

    ///Find Current Input Device
    int getTouchDevice() {
        int fd = -1;
        struct dirent *entry;
        const char *input_path = "/dev/input";

        DIR *dir = opendir(input_path);
        if(!dir) {
            return -1;
        }

        while((entry = readdir(dir))) {
            if (!strstr(entry->d_name, "event"))
                continue;

            char devname[PATH_MAX];
            snprintf(devname, sizeof(devname), "%s/%s", input_path, entry->d_name);

            if(!isCharDevice(devname)){
                continue;
            }

            fd = open(devname, O_RDONLY);
            if(fd < 0) {
                continue;
            }

            if(!HasSpecificAbs(fd, ABS_MT_SLOT)) {
                continue;
            }

            if(!HasSpecificAbs(fd, ABS_MT_TRACKING_ID)){
                continue;
            }

            if(!HasSpecificAbs(fd, ABS_MT_POSITION_X)) {
                continue;
            }

            if(!HasSpecificProp(fd, INPUT_PROP_DIRECT)) {
                continue;
            }

            break;
        }

        closedir(dir);

        return fd;
    }

    ///Write Input Event to Specified Fd
    void writeEvent(int ifd, int type, int code, int value) {
        struct input_event inputEvent = {};

        inputEvent.type = type;
        inputEvent.code = code;
        inputEvent.value = value;

        write(ifd, &inputEvent, sizeof(input_event));
    }

    ///----------Fake Touch Input-----------///
    ///Touch Contact Struct
    struct TouchContact {
        int posX = -1;
        int posY = -1;
        bool enabled = false;
    };

    #define FAKE_CONTACT 9
    int touchXMin;
	int touchXMax;
	int touchYMin;
	int touchYMax;
    int maxContacts;
    int displayWidth;
    int displayHeight;
    int touchDeviceFd = -1;
    int uInputTouchFd = -1;
    int swipeInterval = 10;
    int swipeFrameTime = 15;
    bool isBtnDown = false;
    bool touchSend = false;
    bool touchStart = false;
    std::mutex touchSynMtx;
    Channel<bool> touchChannel;
    struct TouchContact *contacts = NULL;

    ///Reading Touch Inputs
    void eventReaderThread(int fd) {
        int currSlot = 0;
        bool hasSyn = false;
        struct input_event evt;

        //Check TouchChannel
        if(touchChannel.is_closed()){
            return;
        }

        //printf("-------------------------------------\n");

        while (read(fd, &evt, sizeof(evt))) {
            //Contact Data Sync
            touchSynMtx.lock();

            switch (evt.type) {
                case EV_SYN:
                    if(evt.code == SYN_REPORT){
                        hasSyn = true;
                        //printf("SYN_REPORT\n");
                    }
                    break;
                case EV_KEY:
                    if (evt.code == BTN_TOUCH) {
                        //printf("BTN_TOUCH: %s\n", (evt.value == 1) ? "DOWN" : "UP");
                    }
                    break;
                case EV_ABS:
                    switch (evt.code) {
                        case ABS_MT_SLOT:
                            currSlot = evt.value;
                            //printf("ABS_MT_SLOT: %d\n", evt.value);
                            break;
                        case ABS_MT_TRACKING_ID:
                            contacts[currSlot].enabled = evt.value != -1;
                            //printf("ABS_MT_TRACKING_ID: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                        case ABS_MT_POSITION_X:
                            contacts[currSlot].posX = evt.value;
                            //printf("ABS_MT_POSITION_X: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                        case ABS_MT_POSITION_Y:
                            contacts[currSlot].posY = evt.value;
                            //printf("ABS_MT_POSITION_Y: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                    }
                    break;
            }

            touchSynMtx.unlock();

            if (hasSyn) {
                touchChannel.put(true);
                hasSyn = false;
                //printf("-------------------------------------\n");
            }
        }
    }

    ///Writing Touch Inputs
    void eventWriterThread(int ufd){
        while(!touchChannel.is_closed()){
            bool sync;
            while(touchChannel.get(sync) && sync){
                //Contact Data Sync
                touchSynMtx.lock();

                int nextSlot = 0;

                for (int i = 0; i < maxContacts; i++) {
                    if(contacts[i].enabled && contacts[i].posX > 0 && contacts[i].posY > 0){
                        writeEvent(ufd, EV_ABS, ABS_MT_POSITION_X, contacts[i].posX);
                        writeEvent(ufd, EV_ABS, ABS_MT_POSITION_Y, contacts[i].posY);
                        writeEvent(ufd, EV_ABS, ABS_MT_TRACKING_ID, i);
                        writeEvent(ufd, EV_SYN, SYN_MT_REPORT, 0x0);

                        nextSlot++;
                    }
                }

                if(nextSlot == 0 && isBtnDown){ //Button Up
                    isBtnDown = false;
                    writeEvent(ufd, EV_SYN, SYN_MT_REPORT, 0x0);
                    writeEvent(ufd, EV_KEY, BTN_TOUCH, 0x0);
                } else if(nextSlot == 1 && !isBtnDown){ //Button Down
                    isBtnDown = true;
                    writeEvent(ufd, EV_KEY, BTN_TOUCH, 0x1);
                }

                writeEvent(ufd, EV_SYN, SYN_REPORT, 0x0);

                touchSynMtx.unlock();
            }
        }
    }

    void sendTouchMove(int x, int y) {
        if(!touchStart){
            return;
        }

        if(!touchSend){
            touchSend = true;
        }

        //Contact Data Sync
        touchSynMtx.lock();

        contacts[FAKE_CONTACT].posX = (x * touchXMax / displayWidth) + touchXMin;
        contacts[FAKE_CONTACT].posY = (y * touchYMax / displayHeight) + touchYMin;
        contacts[FAKE_CONTACT].enabled = true;

        touchSynMtx.unlock();

        touchChannel.put(true);
    }

    void sendTouchUp() {
        if(!touchStart || !touchSend){
            return;
        }

        touchSend = false;

        //Contact Data Sync
        touchSynMtx.lock();

        contacts[FAKE_CONTACT].posX = -1;
        contacts[FAKE_CONTACT].posY = -1;
        contacts[FAKE_CONTACT].enabled = false;

        touchSynMtx.unlock();

        touchChannel.put(true);
    }

    void touchInputStart(int width, int height) {
        if(!touchStart){
            touchDeviceFd = getTouchDevice();

            if(touchDeviceFd < 0){
                perror("Unable to find touch device\n");
                return;
            } else {
                uInputTouchFd = createUInput(touchDeviceFd);

                if(uInputTouchFd < 0){
                    perror("Unable to create virtual touch device\n");
                    return;
                }

                //Init Things
                displayWidth = width;
                displayHeight = height;

                touchXMin = absX.minimum;
                touchXMax = absX.maximum - absX.minimum + 1;
                touchYMin = absY.minimum;
                touchYMax = absY.maximum - absY.minimum + 1;

                //Set Default Values in Touch Contacts Array
                maxContacts = absSlot.maximum + 1;
                contacts = (struct TouchContact *)malloc((maxContacts) * sizeof(struct TouchContact));
                for (int i = 0; i < maxContacts; i++) {
                    contacts[i].posX = -1;
                    contacts[i].posY = -1;
                    contacts[i].enabled = false;
                }

                std::thread(eventReaderThread, touchDeviceFd).detach();
                std::thread(eventWriterThread, uInputTouchFd).detach();

                touchStart = true;
            }
        }
    }

    void touchInputStop() {
        if(touchStart && touchDeviceFd > -1 && uInputTouchFd > -1){
            touchChannel.close();

            close(touchDeviceFd);
            close(uInputTouchFd);
            free(contacts);

            touchDeviceFd = -1;
            uInputTouchFd = -1;
            contacts = NULL;

            touchStart = false;
        }
    }

    ///Touch Test Code
    void genMovePoints(int StartX, int StartY, int EndX, int EndY){
        int minPointCount = 2;
        int maxMoveDistance = 10;

        float dX = EndX - StartX;
        float dY = EndY - StartY;

        int xCount = abs((int)dX / maxMoveDistance);
        int yCount = abs((int)dY / maxMoveDistance);

        int count = (xCount > yCount) ? xCount : yCount;
        count = (count > minPointCount) ? count : minPointCount;

        for(int i=0; i < count; i++){
            sendTouchMove((int)(StartX + (dX / count) * i), (int)(StartY + (dY / count) * i));
            std::this_thread::sleep_for(std::chrono::milliseconds(swipeInterval));
        }
    }

    void Swipe(int StartX, int StartY, int EndX, int EndY) {
        sendTouchMove(StartX, StartY);
        std::this_thread::sleep_for(std::chrono::milliseconds(swipeInterval));

        genMovePoints(StartX, StartY, EndX, EndY);

        sendTouchMove(EndX, EndY);
        std::this_thread::sleep_for(std::chrono::milliseconds(swipeInterval));

        sendTouchUp();
        std::this_thread::sleep_for(std::chrono::milliseconds(swipeFrameTime));
    }

    void Test() {
        int x = 746;
        int y = 1064;
        int nx = 400;
        int ny = 1408;

        //Using Common Display Resolution, 2340x1080
        touchInputStart(1080, 2340);//Start TouchInput

        std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        Swipe(x, y, x, ny);

        std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        Swipe(nx, y, x, ny);

        std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        Swipe(x, ny, x, y);

        std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        Swipe(x, ny, nx, y);

        std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        while(1){
        }
    }
}

#endif //TOUCHINPUT_H