export type Role = 'admin' | 'user'

export type CurrentUser = {
  id: number
  username: string
  role: Role
}

export type PanelUser = CurrentUser & {
  isActive: boolean
  createdAt: string
  updatedAt: string
  lastLoginAt: string | null
}

export type SetupStatus = {
  initialized: boolean
}

export type UserResponse = {
  user: CurrentUser
}

export type PanelUserResponse = {
  user: PanelUser
}

export type UsersResponse = {
  users: PanelUser[]
}

export type OKResponse = {
  ok: boolean
}
