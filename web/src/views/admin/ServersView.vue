<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  createServer,
  deleteServer,
  listServers,
  testServer,
  updateServer,
  type Server,
} from '@/api/servers'

const items = ref<Server[]>([])
const loading = ref(false)

type ProbeState = {
  status: 'unknown' | 'checking' | 'ok' | 'fail' | 'unconfigured'
  error?: string
  inbound_count?: number
}

const probeStates = ref<Record<number, ProbeState>>({})

const dialog = ref(false)
const busy = ref(false)
const editing = ref<Server | null>(null)
const form = reactive({
  name: '',
  url: '',
  api_token: '',
  username: '',
  password: '',
  remark: '',
  // For edit mode, blank credentials = "keep existing"
  change_api_token: false,
  change_password: false,
})

async function load() {
  loading.value = true
  try {
    items.value = await listServers()
    void refreshStatuses()
  } finally {
    loading.value = false
  }
}

function credentialsConfigured(s: Server): boolean {
  return s.has_api_token || s.has_password
}

function stateFor(s: Server): ProbeState {
  return probeStates.value[s.id] ?? {
    status: credentialsConfigured(s) ? 'unknown' : 'unconfigured',
  }
}

function statusLabel(s: Server): string {
  const state = stateFor(s)
  switch (state.status) {
    case 'checking':
      return '检测中'
    case 'ok':
      return typeof state.inbound_count === 'number'
        ? `连接正常（${state.inbound_count}）`
        : '连接正常'
    case 'fail':
      return '连接失败'
    case 'unconfigured':
      return '未配置凭据'
    default:
      return '未检测'
  }
}

function statusType(s: Server): 'success' | 'warning' | 'danger' | 'info' {
  switch (stateFor(s).status) {
    case 'ok':
      return 'success'
    case 'checking':
      return 'warning'
    case 'fail':
      return 'danger'
    default:
      return 'info'
  }
}

async function probeServer(s: Server, notify = false) {
  if (!credentialsConfigured(s)) {
    probeStates.value = {
      ...probeStates.value,
      [s.id]: { status: 'unconfigured' },
    }
    if (notify) ElMessage.warning('请先配置 API Token 或用户名密码')
    return
  }
  probeStates.value = {
    ...probeStates.value,
    [s.id]: { status: 'checking' },
  }
  try {
    const r = await testServer(s.id)
    if (r.ok) {
      probeStates.value = {
        ...probeStates.value,
        [s.id]: { status: 'ok', inbound_count: r.inbound_count },
      }
      if (notify) ElMessage.success(`连接 OK，3X-UI 有 ${r.inbound_count} 个 inbound`)
    } else {
      probeStates.value = {
        ...probeStates.value,
        [s.id]: { status: 'fail', error: r.error ?? '未知错误' },
      }
      if (notify) ElMessageBox.alert(r.error ?? '未知错误', '连接失败', { type: 'error' })
    }
  } catch (e: any) {
    const message = e?.response?.data?.error ?? e?.message ?? '未知错误'
    probeStates.value = {
      ...probeStates.value,
      [s.id]: { status: 'fail', error: message },
    }
    if (notify) ElMessageBox.alert(message, '连接失败', { type: 'error' })
  }
}

async function refreshStatuses() {
  await Promise.allSettled(items.value.map((s) => probeServer(s)))
}

function openCreate() {
  editing.value = null
  form.name = ''
  form.url = ''
  form.api_token = ''
  form.username = ''
  form.password = ''
  form.remark = ''
  form.change_api_token = true
  form.change_password = true
  dialog.value = true
}

function openEdit(s: Server) {
  editing.value = s
  form.name = s.name
  form.url = s.url
  form.api_token = ''
  form.username = s.username ?? ''
  form.password = ''
  form.remark = s.remark ?? ''
  form.change_api_token = false
  form.change_password = false
  dialog.value = true
}

async function submit() {
  if (!form.url) {
    ElMessage.warning('URL 必填')
    return
  }
  busy.value = true
  try {
    if (editing.value) {
      const req: Record<string, string> = {
        url: form.url,
        name: form.name,
        username: form.username,
        remark: form.remark,
      }
      if (form.change_api_token) req.api_token = form.api_token
      if (form.change_password) req.password = form.password
      await updateServer(editing.value.id, req)
      ElMessage.success('已保存')
    } else {
      if (!form.name) {
        ElMessage.warning('服务器名称必填')
        return
      }
      await createServer({
        name: form.name,
        url: form.url,
        api_token: form.api_token || undefined,
        username: form.username || undefined,
        password: form.password || undefined,
        remark: form.remark || undefined,
      })
      ElMessage.success('已创建')
    }
    dialog.value = false
    await load()
  } finally {
    busy.value = false
  }
}

const testing = ref<string | null>(null)
async function runTest(s: Server) {
  testing.value = s.name
  try {
    await probeServer(s, true)
  } finally {
    testing.value = null
  }
}

async function confirmDelete(s: Server) {
  await ElMessageBox.confirm(
    `删除服务器 ${s.name}？该服务器必须没有节点引用才能删除。`,
    '确认',
    { type: 'warning' },
  )
  await deleteServer(s.id)
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="psp-page-header">
      <div class="psp-page-title">服务器（3X-UI）</div>
      <el-button type="primary" @click="openCreate">新增服务器</el-button>
    </div>

    <div class="hint">
      节点（inbound）关联到服务器。新增节点时从这里的列表选择。
    </div>

    <el-table v-loading="loading" :data="items" stripe>
      <el-table-column prop="name" label="服务器名称" min-width="160" />
      <el-table-column prop="url" label="URL" min-width="280" />
      <el-table-column label="状态" min-width="180">
        <template #default="{ row }">
          <el-tooltip
            v-if="stateFor(row).error"
            :content="stateFor(row).error"
            placement="top"
          >
            <el-tag :type="statusType(row)" size="small">{{ statusLabel(row) }}</el-tag>
          </el-tooltip>
          <el-tag v-else :type="statusType(row)" size="small">{{ statusLabel(row) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="remark" label="备注" min-width="160" />
      <el-table-column label="操作" width="280">
        <template #default="{ row }">
          <el-button
            size="small"
            :loading="testing === row.name"
            @click="runTest(row)"
          >
            测试连接
          </el-button>
          <el-button size="small" type="primary" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" @click="confirmDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      v-model="dialog"
      :title="editing ? `编辑 ${editing.name}` : '新增服务器'"
      width="520px"
    >
      <el-form label-width="100px">
        <el-form-item label="服务器名称" required>
          <el-input v-model="form.name" placeholder="default / us-west / ..." />
          <div class="hint-small">显示名称，可修改；内部关联使用数据库 ID</div>
        </el-form-item>
        <el-form-item label="URL" required>
          <el-input v-model="form.url" placeholder="https://3x-ui.example.com:54321" />
        </el-form-item>
        <el-form-item label="API Token">
          <div v-if="editing && !form.change_api_token" class="masked-line">
            <span class="muted">{{ editing.has_api_token ? '已配置（保持不变）' : '未配置' }}</span>
            <el-button text type="primary" size="small" @click="form.change_api_token = true">
              修改
            </el-button>
          </div>
          <el-input
            v-else
            v-model="form.api_token"
            type="password"
            show-password
            placeholder="3X-UI API Token（推荐）"
          />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="admin（仅在用密码登录时）" />
        </el-form-item>
        <el-form-item label="密码">
          <div v-if="editing && !form.change_password" class="masked-line">
            <span class="muted">{{ editing.has_password ? '已配置（保持不变）' : '未配置' }}</span>
            <el-button text type="primary" size="small" @click="form.change_password = true">
              修改
            </el-button>
          </div>
          <el-input
            v-else
            v-model="form.password"
            type="password"
            show-password
            placeholder="API Token 优先；填密码作为兜底"
          />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="busy" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.psp-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.psp-page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-main);
}

.hint {
  color: var(--text-muted);
  font-size: 13px;
  margin-bottom: 16px;
}

.hint-small {
  color: var(--text-muted);
  font-size: 12px;
  margin-top: 4px;
}

.masked-line {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 32px;
}

.muted {
  color: var(--text-muted);
}
</style>
