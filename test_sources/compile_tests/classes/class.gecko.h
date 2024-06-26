//Gecko standard library
#include <gecko/gecko.h>
int printf(const string format,...);
typedef struct {
string _str;
} std__types__String;
std__types__String* std__types__String__init(string str);
int std__types__String__size(std__types__String* self);
void std__types__String__printSelf(std__types__String* self);
