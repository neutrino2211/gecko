#include <stdio.h>

char* get_greeting(char* name) {
  char buf[128];
  sprintf(&buf, "Hello, %s", name);

  printf("Address of string from C is 0x%x\n", &buf);
  return &buf;
}