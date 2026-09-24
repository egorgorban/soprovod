export type VacancySource = 'hh' | 'manual'

export interface Vacancy {
  id: number
  source: VacancySource
  hh_id: string | null
  url: string | null
  title: string
  company: string
  salary: string | null
  description: string
  key_skills: string[]
  created_at: string
}

export interface RelevantExperienceItem {
  source: string
  text: string
}

export interface LlmOutput {
  role: string
  years: number
  stack: string[]
  relevant_experience: RelevantExperienceItem[]
  letter: string
}

export type ApplicationStatus = 'generated' | 'filtered_out' | 'failed'

export interface Application {
  id: number
  vacancy_id: number
  filter_passed: boolean
  filter_reason: string | null
  status: ApplicationStatus
  generated_text: string | null
  edited_text: string | null
  llm_output: LlmOutput | null
  model: string | null
  error: string | null
  created_at: string
  updated_at: string
}

export interface ListItem {
  id: number
  title: string
  company: string
  status: ApplicationStatus
  created_at: string
  preview: string
}

export interface ApplicationWithVacancy {
  application: Application
  vacancy: Vacancy
}

export interface ListApplicationsResponse {
  items: ListItem[]
  total: number
}

export interface ApiError {
  error: string
}
