#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <errno.h>
#include <uv.h>

#define CONN_NUM 64000
#define CONNECT_INTERVAL 10
#define WRITE_INTERVAL 30000


#define STATUS_NOT_CONNECT 0
#define STATUS_CONNECTED   1
#define STATUS_READING     2
typedef struct {
    uv_tcp_t ts;
    uv_write_t ws;
    uv_connect_t cs;
    char bs[133];
    char status;
    char read_fail_count;
} conn;

conn c[CONN_NUM];

char* addr_str;

uv_loop_t *loop;
struct sockaddr_in addr;
struct sockaddr_in addr1;
uint32_t msg_len;
size_t counter = 0;

void conn_read_done(uv_stream_t *handle,
                           ssize_t nread,
                           const uv_buf_t *buf) {
    size_t i,j;
    i = (size_t)handle->data;
    j = (size_t)c[i].ws.data;

    if(nread == UV_ENOBUFS) printf("nobuf:%d, %d\n",nread,j);
    //printf("j: %d\n",j);
    if (nread == UV_EOF) {
        c[i].cs.data = (void*)-1;
        c[i].status = STATUS_NOT_CONNECT;
        printf("close handle: %d\n",i);
        if (j != 132) {
            printf("premature closed,readed %d, handle: %d\n",j,i);
        }
        uv_close((uv_handle_t *)handle,NULL);
        return;
    }
    if(nread < 0) {
        perror("read");
        printf("j: %d,%d\n",j,i);
        c[i].status = STATUS_NOT_CONNECT;
        uv_close((uv_handle_t *)handle,NULL);
        return;
    }

    j += nread;
    c[i].ws.data = (void*)j;

    if(c[i].status != STATUS_READING){
        printf("status not reading  i: %d\n",i);
    } 
    if(j==132){
	c[i].status = STATUS_CONNECTED;
        //uv_read_stop((uv_stream_t*)&c[i].ts);
    }
    
    if(j>132){
        printf("read >=132 i: %d\n",i);
        j -= 132;
        c[i].ws.data = (void*)j;
        //uv_read_stop((uv_stream_t*)&c[i].ts);
    }
}

void conn_alloc(uv_handle_t *handle, size_t size, uv_buf_t *buf) {
    size_t i,j;
    i = (size_t)handle->data;
    j = (size_t)c[i].ws.data;
    //printf("conn alloc,readed %d, handle: %d\n",j,i);
    buf->base = (&c[i].bs[j]);
    buf->len = 133-j;
}

void write_cb1(uv_write_t *req, int status) {
    size_t i;
    i = (size_t)req->handle->data;
    if(status != 0) {
         perror("write");
    }
    c[i].ws.data = (void*)0;
    c[i].status = STATUS_READING;
    uv_read_start((uv_stream_t*)&c[i].ts, conn_alloc, conn_read_done);
}

void write_cb(uv_write_t *req, int status) {
    size_t i;
    i = (size_t)req->handle->data;
    if(status != 0) {
         perror("write");
         return;
    }
    uv_buf_t buf;
    buf.base = &(c[i].bs[4]);
    c[i].bs[131] = 10;
    buf.len = 128;
    uv_write(&c[i].ws,(uv_stream_t*)&c[i].ts,&buf,1,write_cb1);
}

void connect_cb(uv_connect_t* req, int status) {
    size_t i;
    i = (size_t)req->handle->data;
    if(i%300 == 0){
        printf("%d\n",i);
    }
    else if(i > 59000){
        printf("%d\n",i);
    }
    if(status != 0) {
         perror("connect");
         return;
    }
    c[i].status = STATUS_CONNECTED;
    c[i].read_fail_count = 0;
    uv_buf_t buf;
    buf.base = (char*)&msg_len;
    buf.len = 4;
    uv_write(&c[i].ws,(uv_stream_t*)&c[i].ts,&buf,1,write_cb);
}

void timer_write_cb(uv_write_t *req, int status) {
    size_t i;
    i = (size_t)req->handle->data;
    c[i].status = STATUS_READING;
    if(status != 0){
        perror("timer_cb");
        printf("timer_cb: %d\n",(size_t)req->handle->data);
    }
}

void timer_cb1(uv_timer_t *handle) {
    counter = 0;
    printf("%d ms timer wakeup\n",WRITE_INTERVAL);
    size_t i,read_fin_counter,read_not_fin_counter,closed_socket;
    read_fin_counter = read_not_fin_counter = closed_socket = 0;
    for(i=0;i<CONN_NUM;i++) {
        if(c[i].status == STATUS_CONNECTED)
        {
            c[i].read_fail_count = 0;
            read_fin_counter++;
            uv_buf_t bufs[2];
            bufs[0].base = (char*)&msg_len;
            bufs[0].len = 4;

            bufs[1].base = &(c[i].bs[4]);
            bufs[1].len = 128;

	    c[i].ws.data = (void*)0;

	    uv_write(&c[i].ws,(uv_stream_t*)&c[i].ts,bufs,2,timer_write_cb);
        } else if(c[i].status == STATUS_READING) {
            c[i].read_fail_count++;
            printf("%d,%d,read_fail_count %d; ",i,(size_t)c[i].ws.data,c[i].read_fail_count);
            read_not_fin_counter++;
        }
        
        if((size_t)c[i].status == STATUS_NOT_CONNECT){
            closed_socket++;
        }
    }
    printf("\n\n");
    printf("%d ms timer finish,read_fin: %d, read not fin: %d, closed socket: %d\n",WRITE_INTERVAL,read_fin_counter,read_not_fin_counter,closed_socket);
}

void timer_cb(uv_timer_t *handle) {
    size_t i,j;
    static size_t fail_count = 0;
    for(i = (size_t)handle->data,j = 0;i < CONN_NUM && j < 10;i++,j++) {
        uv_ip4_addr(addr_str, 1024+i, &addr);
        uv_tcp_bind(&c[i].ts, (const struct sockaddr*)&addr, 0);
        uv_tcp_keepalive(&c[i].ts,1,30);
        c[i].cs.data = (void*)0;
        if (0 != uv_tcp_connect(&c[i].cs,&c[i].ts,(struct sockaddr*)&addr1,connect_cb)) {
            perror("connect_rt");
            sleep(1);
            fail_count++;
            printf("failcount: %d\n",fail_count);
        }
    }
    handle->data = (void*)i;
    if(i == CONN_NUM || fail_count >= 20){
        uv_timer_stop(handle);
        //timer for 100 secs
        uv_timer_start(handle, timer_cb1, WRITE_INTERVAL, WRITE_INTERVAL);
    }
}

int main(int argc,char** argv) {
    loop = uv_default_loop();
    if (argc != 2) {
        printf("./tdial localaddr");
        return -1;
    }

    msg_len = htonl(132);
    //uv_ip4_addr("192.168.8.90", 0, &addr);
    if (UV_EINVAL == uv_ip4_addr(argv[1], 0, &addr)) {
        printf("bind error: %s\n",argv[1]);
        return -1;
    }
    addr_str = argv[1];
    uv_ip4_addr("192.168.3.99", 8999, &addr1);
    size_t i;
    for(i=0;i<CONN_NUM;i++) {
        uv_tcp_init(loop, &c[i].ts);
        c[i].ts.data = (void*)i;
        c[i].status = STATUS_NOT_CONNECT;
    }
    uv_timer_t timer_req;
    timer_req.data = (void*)0;

    uv_timer_init(loop, &timer_req);
    uv_timer_start(&timer_req, timer_cb, CONNECT_INTERVAL, CONNECT_INTERVAL);
    
    return uv_run(loop, UV_RUN_DEFAULT);
}
