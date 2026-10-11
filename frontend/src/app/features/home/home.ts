import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { categories } from './categories.data';

@Component({
  imports: [RouterLink],
  selector: 'app-home',
  styleUrl: './home.scss',
  templateUrl: './home.html',
})
export class Home {
  protected readonly categories = categories;
}
