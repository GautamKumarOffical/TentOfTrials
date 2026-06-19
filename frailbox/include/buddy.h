#ifndef FRAILBOX_BUDDY_H
#define FRAILBOX_BUDDY_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define BUDDY_MIN_BLOCK_SIZE 64U
#define BUDDY_DEFAULT_HEAP_SIZE (1024ULL * 1024ULL)

typedef struct buddy_stats {
    uint64_t total_allocated;
    uint64_t total_freed;
    uint64_t peak_usage;
    uint64_t current_usage;
    uint64_t allocation_count;
    uint64_t deallocation_count;
    uint64_t allocated_blocks;
    uint64_t free_blocks;
    size_t   heap_size;
    size_t   largest_free_block;
    double   fragmentation_ratio;
} buddy_stats_t;

int            buddy_init(size_t heap_size);
void           buddy_shutdown(void);
void          *buddy_alloc(size_t size);
void           buddy_free(void *ptr);
buddy_stats_t  buddy_stats(void);

#ifdef __cplusplus
}
#endif

#endif
