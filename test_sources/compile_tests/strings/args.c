#include <stdio.h>

char* get_some() {
  char* r = "Hello";
  return r;
}

int main(int argc, char** argv) {
  char* names[3] = {"One", "Two", get_some()};
  printf("name: %s, arr[0]: %s\n", argv[3], names[1]);
}