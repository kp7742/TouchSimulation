//
// Created by Kuldip Patel on 11/01/25.
//

#include "TouchInput.hpp"

#define FAKE_CONTACT 9

namespace TouchInput {
    const char letterBytes[] = "abcdefghijklmnopqrstuvwxyz";

    int displayWidth;
    int displayHeight;

    int uInputTouchFd = -1;
    bool isBtnDown = false;
    bool isStopped = false;
    bool touchSend = false;
    bool touchStart = false;
    std::mutex touchSynMtx;
    InputDevice touchDevice{};
    vector<struct TouchContact> contacts;

    string randString(int n) {
        string b;

        b.resize(n);

        for (int i = 0; i < n; i++) {
            b[i] = letterBytes[rand() % (sizeof(letterBytes) - 1)];
        }

        return b;
    }

    int randNum(int n) {
        // Generate a random number from 0 to n - 1.
        return rand() % n;
    }

    ///Determine if a path exist and is a character input device.
    int isCharDevice(string &path) {
        struct stat st{};
        if (stat(path.c_str(), &st) == -1) {
            return 0; // Not a character device or does not exist
        }

        if (S_ISCHR(st.st_mode)) {
            return 1; // Is a character device
        }

        return 0; // Not a character device
    }

    /// Determine if a evbits has specified Event Type.
    bool hasSpecificType(InputDevice &id, unsigned int prop) {
        return id.evbits[prop / 8] & (1 << (prop % 8));
    }

    /// Determine if a propbits has specified Input Prop.
    bool hasSpecificProp(InputDevice &id, unsigned int prop) {
        return id.propBits[prop / 8] & (1 << (prop % 8));
    }

    /// Determine if a absbits has specified Abs Key.
    bool hasSpecificAbs(InputDevice &id, unsigned int key) {
        return id.absBits[key / 8] & (1 << (key % 8));
    }

    /// Determine if a keybits has specified Key.
    bool hasSpecificKey(InputDevice &id, unsigned int key) {
        return id.keyBits[key / 8] & (1 << (key % 8));
    }

    /// Read Input Device's ABS Data
    struct input_absinfo getAbsInfo(int ifd, unsigned int key){
        struct input_absinfo absinfo{};

        ioctl(ifd, EVIOCGABS(key), &absinfo);

        return absinfo;
    }

    /// Read Input Device's InputID
    struct input_id getInputID(int ifd){
        struct input_id iid{};

        ioctl(ifd, EVIOCGID, &iid);

        return iid;
    }

    /// Read Event's Device Name
    string getDeviceName(int ifd){
        string name;

        name.resize(UINPUT_MAX_NAME_SIZE);

        ioctl(ifd, EVIOCGNAME(UINPUT_MAX_NAME_SIZE), name.data());

        return name;
    }

    /// Read Event's Physical Location
    string getPhysLoc(int ifd){
        string phys;

        phys.resize(UINPUT_MAX_NAME_SIZE);

        ioctl(ifd, EVIOCGPHYS(UINPUT_MAX_NAME_SIZE), phys.data());

        return phys;
    }

    ///Find Current Input Device
    vector<InputDevice> getTouchDevice() {
        struct dirent *entry;
        string input_path = "/dev/input";

        DIR *dir = opendir(input_path.c_str());
        if(!dir) {
            return {};
        }

        vector<InputDevice> devs;

        while((entry = readdir(dir))) {
            if (!strstr(entry->d_name, "event"))
                continue;

            string devname = input_path + "/" + entry->d_name;

            if(!isCharDevice(devname)){
                continue;
            }

            int ifd = open(devname.c_str(), O_RDONLY);
            if(ifd < 0) {
                continue;
            }

            InputDevice id{};

            // Read Ev data
            if(ioctl(ifd, EVIOCGBIT(0, sizeof(id.evbits)), &id.evbits) < 0){
                continue;
            }

            // Read Abs data
            if(ioctl(ifd, EVIOCGBIT(EV_ABS, sizeof(id.absBits)), &id.absBits) < 0){
                continue;
            }

            // Read Abs data
            if(ioctl(ifd, EVIOCGPROP(sizeof(id.propBits)), &id.propBits) < 0){
                continue;
            }

            // Read Key data
            if(ioctl(ifd, EVIOCGBIT(EV_KEY, sizeof(id.keyBits)), &id.keyBits) < 0){
                continue;
            }

            // Devices with ABS_MT_SLOT - 1 aren't MT devices, libevdev:libevdev.c#L319
            if(hasSpecificAbs(id, ABS_MT_SLOT - 1) || !hasSpecificAbs(id, ABS_MT_SLOT) ||
               !hasSpecificAbs(id, ABS_MT_TRACKING_ID) || !hasSpecificAbs(id, ABS_MT_POSITION_X) ||
               !hasSpecificAbs(id, ABS_MT_POSITION_Y) || !hasSpecificProp(id, INPUT_PROP_DIRECT) ||
               !hasSpecificKey(id, BTN_TOUCH)) {
                continue;
            }

            // Read all AbsInfos
            id.absInfos.resize(ABS_CNT);
            for(unsigned int i = 0; i < ABS_CNT; i++){
                if(!hasSpecificAbs(id, i)){
                    continue;
                }

                auto absInfo = getAbsInfo(ifd, i);

                switch(i){
                    case ABS_MT_SLOT:
                        id.slots = absInfo.maximum + 1;
                        break;
                    case ABS_MT_TRACKING_ID:
                        if(absInfo.minimum == absInfo.maximum){
                            absInfo.minimum = -1;
                            absInfo.maximum = 0xFFFF;
                        }
                        break;
                    case ABS_MT_POSITION_X:
                        id.touchXMin = absInfo.minimum;
                        id.touchXMax = absInfo.maximum - absInfo.minimum + 1;
                        break;
                    case ABS_MT_POSITION_Y:
                        id.touchYMin = absInfo.minimum;
                        id.touchYMax = absInfo.maximum - absInfo.minimum + 1;
                        break;
                    default:
                        break;
                }

                id.absInfos[i] = absInfo;
            }

            // Read InputID
            id.iid = getInputID(ifd);

            // Read Driver Version
            if(ioctl(ifd, EVIOCGVERSION, &id.version) < 0){
                continue;
            }

            // Read other details
            id.path = devname;
            id.phys = getPhysLoc(ifd);
            id.name = getDeviceName(ifd);
            id.hasTouchMajor = hasSpecificAbs(id, ABS_MT_TOUCH_MAJOR);
            id.hasTouchMinor = hasSpecificAbs(id, ABS_MT_TOUCH_MINOR);
            id.hasWidthMajor = hasSpecificAbs(id, ABS_MT_WIDTH_MAJOR);
            id.hasWidthMinor = hasSpecificAbs(id, ABS_MT_WIDTH_MINOR);
            id.hasOrientation = hasSpecificAbs(id, ABS_MT_ORIENTATION);
            id.hasPressure = hasSpecificAbs(id, ABS_MT_PRESSURE);

            devs.push_back(id);

            close(ifd);
        }

        closedir(dir);

        return devs;
    }

    ///Find Touch Device and Create new UInput
    int createUInput(InputDevice &dev) {
        int ufd;

        //Open UInput
        ufd = open("/dev/uinput", O_WRONLY | O_NONBLOCK);
        if (ufd < 0) {
            perror("Unable to open uinput\n");
            return -1;
        }

        // Open Input
        dev.ifd = open(dev.path.c_str(), O_RDONLY);
        if(dev.ifd < 0) {
            perror("Unable to open touch device\n");
            return -1;
        }

        //Setup EV_KEY
        ioctl(ufd, UI_SET_EVBIT, EV_KEY);
        for(unsigned int key = 0; key < KEY_CNT; key++){
            if(hasSpecificKey(dev, key)){
                ioctl(ufd, UI_SET_KEYBIT, key);
            }
        }
        //ioctl(ufd, UI_SET_KEYBIT, BTN_TOUCH);

        //Setup User Device
        struct uinput_user_dev uidev{};
        memset(&uidev, 0, sizeof(uidev));

        string devName = dev.name;
        string randStr = "_" + randString(4);
        strncat((char*) devName.data(), randStr.data(), randStr.size());

        strncpy(uidev.name, devName.data(), UINPUT_MAX_NAME_SIZE);

        // uidev.id.bustype = 0x1C; //BUS_SPI
        // uidev.id.vendor = randNum(0x2000);
        // uidev.id.product = randNum(0x2000);
        // uidev.id.version = randNum(0x200);

        uidev.id.bustype = dev.iid.bustype;
        uidev.id.vendor = dev.iid.version;
        uidev.id.product = dev.iid.product;
        uidev.id.version = dev.iid.version;

        uidev.ff_effects_max = 0;
        if(hasSpecificType(dev, EV_FF)){
            uidev.ff_effects_max = 10;
        }

        //Setup EV_ABS
        ioctl(ufd, UI_SET_EVBIT, EV_ABS);

        for(unsigned int abs = 0; abs < ABS_CNT; abs++){
            if(!hasSpecificAbs(dev, abs)){
                continue;
            }

            // Make Sure only TypeA enabled ABS got set
            // absMtPositionX, absMtPositionY, absMtTrackingId
            if(abs == ABS_MT_POSITION_X || abs == ABS_MT_POSITION_Y || abs == ABS_MT_TRACKING_ID){
                ioctl(ufd, UI_SET_ABSBIT, abs);
                uidev.absmin[abs] = dev.absInfos[abs].minimum;
                uidev.absmax[abs] = dev.absInfos[abs].maximum;
                uidev.absfuzz[abs] = dev.absInfos[abs].fuzz;
                uidev.absflat[abs] = dev.absInfos[abs].flat;
            }
        }

        //Setup Input Props
        for(unsigned int prop = 0; prop < INPUT_PROP_CNT; prop++){
            if(hasSpecificProp(dev, prop)){
                ioctl(ufd, UI_SET_PROPBIT, prop);
            }
        }
        // ioctl(ufd, UI_SET_PROPBIT, INPUT_PROP_DIRECT);

        // Setup physical dev name
        ioctl(ufd, UI_SET_PHYS, dev.phys.c_str());

        //Write to Input Sub-System
        write(ufd, &uidev, sizeof(uidev));

        //Declare Input Device
        if(ioctl(ufd, UI_DEV_CREATE) < 0){
            perror("Unable to create uinput\n");
            return -1;
        }

        std::this_thread::sleep_for(std::chrono::milliseconds(200));

        //Stop Primary Touch Device
        ioctl(dev.ifd, EVIOCGRAB, 1);

        return ufd;
    }

    ///Write Input Event to Specified Fd
    void writeEvent(int ifd, int type, int code, int value) {
        struct input_event inputEvent = {};

        inputEvent.type = type;
        inputEvent.code = code;
        inputEvent.value = value;

        write(ifd, &inputEvent, sizeof(input_event));
    }

    ///Writing Touch Inputs
    void eventUpdate(){
        if(isStopped)
            return;

        int nextSlot = 0;

        for (int i = 0; i < touchDevice.slots; i++) {
            if(contacts[i].enabled && contacts[i].posX > 0 && contacts[i].posY > 0){
                writeEvent(uInputTouchFd, EV_ABS, ABS_MT_POSITION_X, contacts[i].posX);
                writeEvent(uInputTouchFd, EV_ABS, ABS_MT_POSITION_Y, contacts[i].posY);
                writeEvent(uInputTouchFd, EV_ABS, ABS_MT_TRACKING_ID, i);
                writeEvent(uInputTouchFd, EV_SYN, SYN_MT_REPORT, 0x0);

                nextSlot++;
            }
        }

        if(nextSlot == 0 && isBtnDown){ //Button Up
            isBtnDown = false;
            writeEvent(uInputTouchFd, EV_SYN, SYN_MT_REPORT, 0x0);
            writeEvent(uInputTouchFd, EV_KEY, BTN_TOUCH, 0x0);
        } else if(nextSlot == 1 && !isBtnDown){ //Button Down
            isBtnDown = true;
            writeEvent(uInputTouchFd, EV_KEY, BTN_TOUCH, 0x1);
        }

        writeEvent(uInputTouchFd, EV_SYN, SYN_REPORT, 0x0);
    }

    ///Reading Touch Inputs
    void eventReaderThread() {
        int currSlot = 0;
        bool hasSyn = false;
        struct input_event evt{};

        // printf("-------------------------------------\n");

        while(!isStopped && read(touchDevice.ifd, &evt, sizeof(evt))) {
            //Contact Data Sync
            touchSynMtx.lock();

            switch (evt.type) {
                case EV_SYN:
                    if(evt.code == SYN_REPORT){
                        hasSyn = true;
                        // printf("SYN_REPORT\n");
                    }
                    break;
                case EV_KEY:
                    if (evt.code == BTN_TOUCH) {
                        // printf("BTN_TOUCH: %s\n", (evt.value == 1) ? "DOWN" : "UP");
                    }
                    break;
                case EV_ABS:
                    switch (evt.code) {
                        case ABS_MT_SLOT:
                            currSlot = evt.value;
                            // printf("ABS_MT_SLOT: %d\n", evt.value);
                            break;
                        case ABS_MT_TRACKING_ID:
                            contacts[currSlot].enabled = evt.value != -1;
                            // printf("ABS_MT_TRACKING_ID: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                        case ABS_MT_POSITION_X:
                            contacts[currSlot].posX = evt.value;
                            // printf("ABS_MT_POSITION_X: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                        case ABS_MT_POSITION_Y:
                            contacts[currSlot].posY = evt.value;
                            // printf("ABS_MT_POSITION_Y: %d | Slot: %d\n", evt.value, currSlot);
                            break;
                    }
                    break;
            }

            if (hasSyn) {
                eventUpdate();
                hasSyn = false;
                // printf("-------------------------------------\n");
            }

            touchSynMtx.unlock();
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

        contacts[FAKE_CONTACT].posX = (x * touchDevice.touchXMax / displayWidth) + touchDevice.touchXMin;
        contacts[FAKE_CONTACT].posY = (y * touchDevice.touchYMax / displayHeight) + touchDevice.touchYMin;
        contacts[FAKE_CONTACT].enabled = true;

        eventUpdate();

        touchSynMtx.unlock();
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

        eventUpdate();

        touchSynMtx.unlock();
    }

    void resetTouch() {
        touchSend = false;

        //Contact Data Sync
        touchSynMtx.lock();

        for (int i = 0; i < touchDevice.slots; i++) {
            contacts[i].posX = -1;
            contacts[i].posY = -1;
            contacts[i].enabled = false;
        }

        touchSynMtx.unlock();

        eventUpdate();
    }

    void updateRes(int x, int y){
        displayWidth = x;
        displayHeight = y;
    }

    void touchInputStart() {
        if(!touchStart){
            auto devs = getTouchDevice();
            if(devs.empty()){
                perror("Devices are not found\n");
                return;
            }

            if(devs.size() > 1){
                // Give it a try
                touchDevice = devs[1];
            } else {
                touchDevice = devs[0];
            }

            uInputTouchFd = createUInput(touchDevice);

            if(uInputTouchFd < 0){
                perror("Unable to create virtual touch device\n");
                return;
            }

            //Set Default Values in Touch Contacts Array
            contacts.clear();
            contacts.resize(touchDevice.slots);
            for (int i = 0; i < touchDevice.slots; i++) {
                contacts[i].posX = -1;
                contacts[i].posY = -1;
                contacts[i].enabled = false;
            }

            std::thread(eventReaderThread).detach();

            touchStart = true;
        }
    }

    void touchInputStop() {
        if(touchStart && touchDevice.ifd > -1 && uInputTouchFd > -1){
            resetTouch();

            isStopped = true;

            std::this_thread::sleep_for(std::chrono::milliseconds(60));

            close(touchDevice.ifd);
            close(uInputTouchFd);

            touchDevice.ifd = -1;
            uInputTouchFd = -1;

            touchStart = false;
        }
    }

    ///Touch Test Code
    int swipeInterval = 10;
    int swipeFrameTime = 15;

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
        updateRes(1080, 2340);
        touchInputStart();//Start TouchInput

        // std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        // Swipe(x, y, x, ny);

        // std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        // Swipe(nx, y, x, ny);

        // std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        // Swipe(x, ny, x, y);

        // std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        // Swipe(x, ny, nx, y);

        // std::this_thread::sleep_for(std::chrono::milliseconds(3000));

        while(1){
        }
    }
}