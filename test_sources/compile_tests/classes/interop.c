#include "./class.gecko.h"
#include <stdio.h>

void std__types__String__printSelf(std__types__String* str) {
  printf("My string '%s'\n", str->_str);
}

int main() {
  std__types__String* str = std__types__String__init("Hello world");
  str->_str = "Second Hello World";
  int sizeOfString = std__types__String__size(str);

  std__types__String__printSelf(str);
  printf("Size of Gecko String %d\n", sizeOfString);
}