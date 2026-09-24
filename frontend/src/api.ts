import type {
  Application,
  ApplicationWithVacancy,
  ListApplicationsResponse,
  ListItem,
  Vacancy,
} from './types'

export class ApiRequestError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  if (!res.ok) {
    let message = `Ошибка запроса (${res.status})`
    try {
      const body = await res.json()
      if (body?.error) message = body.error
    } catch {
      // ignore
    }
    throw new ApiRequestError(message, res.status)
  }
  return res.json() as Promise<T>
}

export type CreateApplicationInput = { url: string } | { text: string; title?: string; company?: string }

export interface Api {
  createApplication(input: CreateApplicationInput): Promise<ApplicationWithVacancy>
  listApplications(limit: number, offset: number): Promise<ListApplicationsResponse>
  getApplication(id: number): Promise<ApplicationWithVacancy>
  updateApplication(id: number, editedText: string): Promise<{ application: Application }>
}

const realApi: Api = {
  createApplication(input) {
    return request<ApplicationWithVacancy>('/api/applications', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  listApplications(limit, offset) {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) })
    return request<ListApplicationsResponse>(`/api/applications?${params.toString()}`)
  },
  getApplication(id) {
    return request<ApplicationWithVacancy>(`/api/applications/${id}`)
  },
  updateApplication(id, editedText) {
    return request<{ application: Application }>(`/api/applications/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ edited_text: editedText }),
    })
  },
}

// ---- Mock mode (VITE_MOCK=1) ----

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

let mockAutoId = 1000
const mockStore = new Map<number, ApplicationWithVacancy>()

function firstLine(text: string, maxLen = 120): string {
  const line = text.split('\n').map((l) => l.trim()).find((l) => l !== '') ?? ''
  return line.length > maxLen ? line.slice(0, maxLen) : line
}

function makeMockVacancy(input: CreateApplicationInput): Vacancy {
  const id = mockAutoId++
  if ('text' in input) {
    return {
      id,
      source: 'manual',
      hh_id: null,
      url: null,
      title: input.title?.trim() || firstLine(input.text),
      company: input.company?.trim() ?? '',
      salary: null,
      description: input.text.trim(),
      key_skills: [],
      created_at: new Date().toISOString(),
    }
  }
  return {
    id,
    source: 'hh',
    hh_id: String(1000000 + id),
    url: input.url,
    title: 'Backend-разработчик (Go)',
    company: 'ООО «Ромашка»',
    salary: 'от 250 000 до 350 000 ₽ на руки',
    description:
      'Мы ищем опытного backend-разработчика в команду платёжной платформы.\n\n' +
      'Обязанности:\n— Разработка и поддержка микросервисов на Go\n— Проектирование API\n— Code review\n\n' +
      'Требования:\n— Опыт коммерческой разработки на Go от 3 лет\n— Знание PostgreSQL, Kafka\n— Понимание принципов проектирования распределённых систем',
    key_skills: ['Go', 'PostgreSQL', 'Kafka', 'Docker', 'gRPC'],
    created_at: new Date().toISOString(),
  }
}

function makeMockApplication(vacancyId: number): Application {
  const id = mockAutoId++
  const now = new Date().toISOString()
  const letter =
    'Добрый день!\n\n' +
    'Я backend-разработчик с опытом 5+ лет. Основной стек: Go, PostgreSQL, Kafka.\n' +
    'Сейчас ищу предложения, связанные с Go и высоконагруженными сервисами, поэтому заинтересовала ваша вакансия.\n\n' +
    'В Sberlabs разрабатывал API-Gateway на Go, выдерживающий несколько тысяч RPS. ' +
    'В Х5 Tech строил интеграции с Kafka для обработки событий заказов.\n\n' +
    'Буду рад обсудить дальнейшие шаги!'
  return {
    id,
    vacancy_id: vacancyId,
    filter_passed: true,
    filter_reason: 'mock',
    status: 'generated',
    generated_text: letter,
    edited_text: null,
    llm_output: {
      role: 'Backend-разработчик',
      years: 5,
      stack: ['Go', 'PostgreSQL', 'Kafka'],
      relevant_experience: [
        { source: 'Sberlabs', text: 'разрабатывал API-Gateway на Go, выдерживающий несколько тысяч RPS' },
        { source: 'Х5 Tech', text: 'строил интеграции с Kafka для обработки событий заказов' },
      ],
      letter,
    },
    model: 'gpt-4.1-nano (mock)',
    error: null,
    created_at: now,
    updated_at: now,
  }
}

const mockApi: Api = {
  async createApplication(input) {
    await delay(2500)
    const vacancy = makeMockVacancy(input)
    const application = makeMockApplication(vacancy.id)
    mockStore.set(application.id, { application, vacancy })
    return { application, vacancy }
  },
  async listApplications(limit, offset) {
    await delay(300)
    const all = Array.from(mockStore.values())
      .sort((a, b) => b.application.id - a.application.id)
      .map<ListItem>(({ application, vacancy }) => ({
        id: application.id,
        title: vacancy.title,
        company: vacancy.company,
        status: application.status,
        created_at: application.created_at,
        preview: (application.edited_text ?? application.generated_text ?? '').slice(0, 150),
      }))
    return { items: all.slice(offset, offset + limit), total: all.length }
  },
  async getApplication(id) {
    await delay(300)
    const found = mockStore.get(id)
    if (!found) throw new ApiRequestError('Заявка не найдена', 404)
    return found
  },
  async updateApplication(id, editedText) {
    await delay(300)
    const found = mockStore.get(id)
    if (!found) throw new ApiRequestError('Заявка не найдена', 404)
    found.application = { ...found.application, edited_text: editedText, updated_at: new Date().toISOString() }
    mockStore.set(id, found)
    return { application: found.application }
  },
}

export const api: Api = import.meta.env.VITE_MOCK === '1' ? mockApi : realApi
