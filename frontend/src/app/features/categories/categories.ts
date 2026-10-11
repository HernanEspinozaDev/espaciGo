import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { categories } from '../home/categories.data';

@Component({
  imports: [RouterLink],
  selector: 'app-categories',
  styleUrl: './categories.scss',
  templateUrl: './categories.html',
})
export class Categories {
  protected readonly categories = categories;
}
