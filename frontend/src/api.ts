import axios from 'axios'

export const api = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

export type Record = { [key: string]: any }
