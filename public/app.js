const store = new Vuex.Store({
  state: {
    messages: {}
  },
  mutations: {
    sync (state, messages) {
      state.messages = messages
    },
    add (state, { id, message }) {
      Vue.set(state.messages, id, message)
    },
    del (state, id) {
      Vue.delete(state.messages, id)
    }
  }
})

Vue.component('message', {
  filters: {
    prettyJson: function (data) {
      data = JSON.parse(JSON.stringify(data))
      for (let key of ['from', 'to', 'html', 'text', 'subject']) {
        delete data[key]
      }
      return JSON.stringify(data, null, ' ')
    }
  },
  props: {
    id: {
      type: String,
      required: true
    },
    msg: {
      type: Object,
      required: true
    }
  },
  data: function () {
    return {
      open: false
    }
  },
  methods: {
    modal (id) {
      let w = window.document.open('', 'html', 'width=500, height=400')
      w.document.write(this.$store.state.messages[id]['html'])
    },
    action (data) {
      if (ws !== null) {
        ws.send(JSON.stringify({ ...data }))
      }
    }
  },
  template: `
<tr>
    <td>{{ msg.to.join(', ') }}</td>
    <td>{{ msg.subject.join(', ') }}</td>
    <td>
        <button id="show-modal" @click="open = true">Show</button>
        <div v-if="open" class="modal-mask">
            <div class="modal-wrapper">
                <div class="modal-container">
                    <table border="1">
                        <tr>
                            <td>Message-Id</td>
                            <td>{{ id }}</td>
                        </tr>
                        <tr>
                            <td>from</td>
                            <td>{{ msg.from.join(', ') }}</td>
                        </tr>
                        <tr>
                            <td>to</td>
                            <td>{{ msg.to.join(', ') }}</td>
                        </tr>
                        <tr>
                            <td>subject</td>
                            <td>{{ msg.subject.join(', ') }}</td>
                        </tr>
                        <tr v-if="msg.text">
                            <td>text</td>
                            <td style="white-space: pre-wrap;">{{ msg.text.join(', ') }}</td>
                        </tr>
                        <tr v-if="msg.html">
                            <td>html</td>
                            <td>
                                <button style="border-bottom: 1px dashed #ccc; cursor: pointer;" @click.prevent="modal(id)">Open in a new window</button>
                            </td>
                        </tr>
                        <tr>
                            <td>Data</td>
                            <td><pre>{{ msg | prettyJson }}</pre></td>
                        </tr>
                        <tr>
                            <td>Actions</td>
                            <td>
                                <button @click.prevent="action({action: 'webhook', event: 'delivered', id})" type="button">Delivered</button>
                                <button @click.prevent="action({action: 'webhookLegacy', event: 'delivered', id})" type="button">Delivered (Legacy)</button>
                                <button @click.prevent="action({action: 'remove', id})" type="button">Remove</button>
                            </td>
                        </tr>
                    </table>
                    <div class="modal-footer">
                        <button class="modal-default-button" @click="open = false">OK</button>
                    </div>
                </div>
            </div>
        </div>
    </td>
</tr>`
})

const app = new Vue({
  store,
  template: `
<table border="1">
    <tr v-if="Object.keys(store.state.messages).length === 0"><td>Queue is empty</td></tr>
    <template v-else>
        <message v-for="(msg, id) in store.state.messages" v-bind:key="id" v-bind:id="id" v-bind:msg="msg"></message>
    </template>
</table>`
})

app.$mount('#app')

/** @type {WebSocket|null} */
let ws = null

connect = function () {
  ws = new WebSocket('ws' + window.location.protocol.substring(4) + '//' + window.location.host + '/ws')

  ws.onclose = function () {
    ws = null
  }

  ws.onmessage = function (ev) {
    /** @type {{action: string, id?: string, data?: Object}} */
    let data = { action: '' }

    try {
      data = JSON.parse(ev.data)
    } catch (err) {
      console.debug(err)
      return
    }

    switch (data.action) {
      case 'sync':
        store.commit('sync', data.data)
        break
      case 'add':
        store.commit('add', { id: data.id, message: data.data })
        break
      case 'del':
        store.commit('del', data.id)
        break
      default:
        console.debug('unknown action', data)
    }
  }
}

connect()
setInterval(function () {
  if (ws === null) {
    connect()
  }
}, 5000)
